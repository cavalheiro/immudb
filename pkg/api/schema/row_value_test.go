/*
Copyright 2021 CodeNotary, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package schema

import (
	"testing"

	"github.com/codenotary/immudb/embedded/sql"
	"github.com/stretchr/testify/require"
)

func TestRowComparison(t *testing.T) {
	nullValue := &SQLValue_Null{}
	trueValue := &SQLValue_B{B: true}
	falseValue := &SQLValue_B{B: false}
	stringValue1 := &SQLValue_S{S: "string1"}
	stringValue2 := &SQLValue_S{S: "string2"}
	intValue1 := &SQLValue_N{N: 1}
	intValue2 := &SQLValue_N{N: 2}
	blobValue1 := &SQLValue_Bs{Bs: nil}
	blobValue2 := &SQLValue_Bs{Bs: []byte{1, 2, 3}}

	equals, err := nullValue.Equal(nullValue)
	require.NoError(t, err)
	require.True(t, equals)

	equals, err = nullValue.Equal(trueValue)
	require.False(t, equals)

	equals, err = trueValue.Equal(nullValue)
	require.False(t, equals)

	equals, err = trueValue.Equal(stringValue1)
	require.Equal(t, sql.ErrNotComparableValues, err)

	equals, err = trueValue.Equal(falseValue)
	require.NoError(t, err)
	require.False(t, equals)

	equals, err = stringValue1.Equal(nullValue)
	require.False(t, equals)

	_, err = stringValue1.Equal(trueValue)
	require.Equal(t, sql.ErrNotComparableValues, err)

	equals, err = stringValue1.Equal(stringValue2)
	require.NoError(t, err)
	require.False(t, equals)

	equals, err = intValue1.Equal(nullValue)
	require.False(t, equals)

	_, err = intValue1.Equal(trueValue)
	require.Equal(t, sql.ErrNotComparableValues, err)

	equals, err = intValue1.Equal(intValue2)
	require.NoError(t, err)
	require.False(t, equals)

	equals, err = blobValue1.Equal(nullValue)
	require.False(t, equals)

	_, err = blobValue1.Equal(trueValue)
	require.Equal(t, sql.ErrNotComparableValues, err)

	equals, err = blobValue1.Equal(blobValue2)
	require.NoError(t, err)
	require.False(t, equals)

	rawNilValue := RawValue(nil)
	require.Equal(t, nil, rawNilValue)

	rawNullValue := RawValue(&SQLValue{Value: nullValue})
	require.Equal(t, nil, rawNullValue)

	rawTrueValue := RawValue(&SQLValue{Value: trueValue})
	require.Equal(t, true, rawTrueValue)

	rawFalseValue := RawValue(&SQLValue{Value: falseValue})
	require.Equal(t, false, rawFalseValue)

	rawStringValue := RawValue(&SQLValue{Value: stringValue1})
	require.Equal(t, "string1", rawStringValue)

	rawIntValue := RawValue(&SQLValue{Value: intValue1})
	require.Equal(t, uint64(1), rawIntValue)

	rawBlobValue := RawValue(&SQLValue{Value: blobValue2})
	require.Equal(t, []byte{1, 2, 3}, rawBlobValue)
}