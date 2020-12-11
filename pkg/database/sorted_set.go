package database

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/codenotary/immudb/embedded/store"
	"github.com/codenotary/immudb/embedded/tbtree"
	"github.com/codenotary/immudb/pkg/api/schema"
	"github.com/codenotary/immudb/pkg/common"
	"math"
)

// ZAdd adds a score for an existing key in a sorted set
// As a parameter of ZAddOptions is possible to provide the associated index of the provided key. In this way, when resolving reference, the specified version of the key will be returned.
// If the index is not provided the resolution will use only the key and last version of the item will be returned
// If ZAddOptions.index is provided key is optional
func (d *db) ZAdd(zaddOpts *schema.ZAddOptions) (index *schema.Root, err error) {

	ik, referenceValue, err := d.getSortedSetKeyVal(zaddOpts, false)
	if err != nil {
		return nil, err
	}

	id, _, alh, err := d.st.Commit([]*store.KV{{Key: ik, Value: referenceValue}})
	if err != nil {
		return nil, fmt.Errorf("unexpected error %v during %s", err, "Reference")
	}

	return &schema.Root{
		Payload: &schema.RootIndex{
			Index: id,
			Root:  alh[:],
		},
	}, nil
}

// ZScan ...
func (d *db) ZScan(options *schema.ZScanOptions) (*schema.ZItemList, error) {
	/*if len(options.Set) == 0 || isReservedKey(options.Set) {
		return nil, ErrInvalidSet
	}

	if isReservedKey(options.Offset) {
		return nil, ErrInvalidOffset
	}*/

	set := common.WrapSeparatorToSet(options.Set)

	offsetKey := set

	// here we compose the offset if Min score filter is provided only if is not reversed order
	if options.Min != nil && !options.Reverse {
		offsetKey = common.AppendScoreToSet(options.Set, options.Min.Score)
	}
	// here we compose the offset if Max score filter is provided only if is reversed order
	if options.Max != nil && options.Reverse {
		offsetKey = common.AppendScoreToSet(options.Set, options.Max.Score)
	}
	// if offset is provided by client it takes precedence
	if len(options.Offset) > 0 {
		offsetKey = options.Offset
	}

	snapshot, err := d.st.Snapshot()
	if err != nil {
		return nil, err
	}
	defer snapshot.Close()

	reader, err := snapshot.Reader(&tbtree.ReaderSpec{
		IsPrefix:   true,
		InitialKey: offsetKey,
		AscOrder:   options.Reverse})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var items []*schema.ZItem
	i := uint64(0)

	var limit = options.Limit
	if limit == 0 {
		// we're reusing max batch count to enforce the default scan limit
		limit = math.MaxUint64
	}

	for {
		sortedSetItemKey, btreeVal, sortedSetItemIndex, err := reader.Read()
		if err == tbtree.ErrNoMoreEntries {
			break
		}
		if err != nil {
			return nil, err
		}

		valLen := binary.BigEndian.Uint32(btreeVal)
		vOff := binary.BigEndian.Uint64(btreeVal[4:])

		var hVal [sha256.Size]byte
		copy(hVal[:], btreeVal[4+8:])

		refVal := make([]byte, valLen)
		_, err = d.st.ReadValueAt(refVal, int64(vOff), hVal)

		var zitem *schema.ZItem
		var item *schema.Item

		//Reference lookup
		if bytes.HasPrefix(sortedSetItemKey, common.SortedSetSeparator) {

			refKey, flag, refIndex := common.UnwrapIndexReference(refVal)

			// here check for index reference, if present we resolve reference with itemAt
			if flag == byte(1) {
				if err = d.st.ReadTx(refIndex, d.tx); err != nil {
					return nil, err
				}
				val, err := d.st.ReadValue(d.tx, refKey)
				if err != nil {
					return nil, err
				}

				item = &schema.Item{Key: refKey, Value: val, Index: refIndex}
			} else {
				item, err = d.Get(&schema.Key{Key: refKey})
				if err != nil {
					return nil, err
				}
			}
		}

		if item != nil {
			zitem = &schema.ZItem{
				Item:          item,
				Score:         common.SetKeyScore(sortedSetItemKey, options.Set),
				CurrentOffset: sortedSetItemKey,
				Index:         sortedSetItemIndex,
			}
		}

		// Guard to ensure that score match the filter range if filter is provided
		if options.Min != nil && zitem.Score < options.Min.Score {
			continue
		}
		if options.Max != nil && zitem.Score > options.Max.Score {
			continue
		}

		items = append(items, zitem)
		if i++; i == limit {
			break
		}
	}

	list := &schema.ZItemList{
		Items: items,
	}

	return list, nil
}

//SafeZAdd ...
func (d *db) SafeZAdd(opts *schema.SafeZAddOptions) (*schema.Proof, error) {
	//return d.st.SafeZAdd(*opts)
	return nil, fmt.Errorf("Functionality not yet supported: %s", "SafeZAdd")
}

// getSortedSetKeyVal return a key value pair that represent a sorted set entry.
// If skipPersistenceCheck is true and index is not provided reference lookup is disabled.
// This is used in Ops, to enable an key value creation with reference insertion in the same transaction.
func (d *db) getSortedSetKeyVal(zaddOpts *schema.ZAddOptions, skipPersistenceCheck bool) (k, v []byte, err error) {

	var referenceValue []byte
	var index = &schema.Index{}
	var key []byte
	if zaddOpts.Index != nil {
		if !skipPersistenceCheck {
			if err := d.st.ReadTx(zaddOpts.Index.Index, d.tx); err != nil {
				return nil, nil, err
			}
			// check if specific key exists at the referenced index
			if _, err := d.st.ReadValue(d.tx, zaddOpts.Key); err != nil {
				return nil, nil, ErrIndexKeyMismatch
			}
			key = zaddOpts.Key
		} else {
			key = zaddOpts.Key
		}
		// here the index is appended the reference value
		// In case that skipPersistenceCheck == true index need to be assigned carefully
		index = zaddOpts.Index
	} else {
		i, err := d.Get(&schema.Key{Key: zaddOpts.Key})
		if err != nil {
			return nil, nil, err
		}
		if bytes.Compare(i.Key, zaddOpts.Key) != 0 {
			return nil, nil, ErrIndexKeyMismatch
		}
		key = zaddOpts.Key
		// Index has not to be stored inside the reference if not submitted by the client. This is needed to permit verifications in SDKs
		index = nil
	}
	ik := common.BuildSetKey(key, zaddOpts.Set, zaddOpts.Score.Score, index)

	// append the index to the reference. In this way the resolution will be index based
	referenceValue = common.WrapIndexReference(key, index)

	return ik, referenceValue, err
}