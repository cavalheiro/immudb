// +build webconsole
//go:generate go run github.com/rakyll/statik -f -src=./dist

package webconsole

import (
	"github.com/codenotary/immudb/pkg/logger"
	"net/http"
	// embedded static files
	_ "github.com/codenotary/immudb/webconsole/statik"
	"github.com/rakyll/statik/fs"
)

func SetupWebconsole(mux *http.ServeMux, l logger.Logger, addr string) error {
	statikFS, err := fs.New()
	if err != nil {
		return err
	}
	l.Infof("Webconsole enabled: %s", addr)
	mux.Handle("/", http.FileServer(statikFS))
	return nil
}
