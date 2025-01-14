// Package autorefresh provides a mechanism for attaching browser refreshing to your templates during development.
// When it is detected that your program has restarted (e.g., by using a live-reload tool like "air"), it will
// trigger the browser page to refresh itself automatically.
package autorefresh

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const Script string = `
<script>
	function setupReloadSocket(reload = false) {
		const reloadWebsocket = new WebSocket({{ path }});
		let doReloadNext = reload;
		reloadWebsocket.onopen = function () {
			if (reload === true) {
				window.location.reload();
			} else {
				doReloadNext = true;
			}
		};
		reloadWebsocket.onerror = function onError() {
			setTimeout(() => setupReloadSocket(doReloadNext), {{ refreshRate }});
		};
		reloadWebsocket.onclose = function onClose() {
			setTimeout(() => setupReloadSocket(doReloadNext), {{ refreshRate }});
		};
	}
	setupReloadSocket();
</script>

`

type PageReloader struct {
	Template    *template.Template
	Path        string
	RefreshRate uint
}

var (
	ErrInvalidParameters = errors.New("Invalid parameters")
	ErrTemplateParsing   = errors.New("Failed to parse template")
)

func New(t *template.Template, path string, refreshRate uint) (*PageReloader, error) {
	// If there was no template passed, create our own and let it get used in some other way
	if t == nil {
		t = template.New("autorefresh")
	}
	if refreshRate < 100 {
		return nil, fmt.Errorf("%w: refreshRate must be at least 100ms", ErrInvalidParameters)
	}
	t, err := t.Funcs(template.FuncMap{
		"path":        func() string { return path },
		"refreshRate": func() uint { return refreshRate },
	}).Parse(Script)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTemplateParsing, err)
	}
	return &PageReloader{Path: path, Template: t, RefreshRate: refreshRate}, nil
}

func (p *PageReloader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	socket, err := websocket.Accept(w, r, nil)
	if err != nil {
		_, _ = w.Write([]byte("could not open websocket"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer socket.Close(websocket.StatusGoingAway, "server closing websocket")
	ctx := r.Context()
	socketCtx := socket.CloseRead(ctx)
	for {
		_ = socket.Ping(socketCtx)
		time.Sleep(time.Second * 2)
	}
}
