package server

import (
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

// stdioConn wraps os.Stdin and os.Stdout to provide a net.Conn-like interface.
type stdioConn struct {
	r io.Reader // typically os.Stdin
	w io.Writer // typically os.Stdout
}

// Read reads from the underlying reader.
func (c *stdioConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

// Write writes to the underlying writer.
func (c *stdioConn) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

// Close is a dummy close because os.Stdin/Stdout are typically managed by the OS.
func (c *stdioConn) Close() error {
	return nil
}

// LocalAddr returns a dummy local address.
func (c *stdioConn) LocalAddr() net.Addr {
	return dummyAddr("local")
}

// RemoteAddr returns a dummy remote address.
func (c *stdioConn) RemoteAddr() net.Addr {
	return dummyAddr("remote")
}

// SetDeadline is a no-op.
func (c *stdioConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is a no-op.
func (c *stdioConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline is a no-op.
func (c *stdioConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// dummyAddr is a simple implementation of net.Addr.
type dummyAddr string

func (d dummyAddr) Network() string { return string(d) }
func (d dummyAddr) String() string  { return string(d) }

func CreateStdioConn(r io.Reader, w io.Writer) net.Conn {
	return &stdioConn{
		r: r,
		w: w,
	}
}

func Server(conn net.Conn, handler http.Handler) {
	// Create an HTTP/2 server instance with custom configuration (if needed).
	h2Server := &http2.Server{
		MaxConcurrentStreams: 250, // for example, customize as needed
	}

	// // Define the HTTP handler to process requests.
	// handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	// This is a basic example that responds with a plain text message.x
	// 	w.Header().Set("Content-Type", "text/plain")
	// 	w.WriteHeader(http.StatusOK)
	// 	//w.(http.Flusher).Flush()
	// 	fmt.Fprintf(w, "Hello from the custom HTTP/2 transport over stdio!\n%s\n", r.RequestURI)
	// 	io.Copy(w, r.Body)
	// 	// for i := 0; i < 2; i++ {
	// 	// 	w.(http.Flusher).Flush()
	// 	// 	time.Sleep(time.Second)
	// 	// 	fmt.Fprintf(w, "ping %d / %s!\n", i, base64.StdEncoding.EncodeToString(rnd))
	// 	// 	//fmt.Printf("ping %d / %s!\n", i, base64.StdEncoding.EncodeToString(rnd))
	// 	// }
	// })

	// Use http2.Server's ServeConn to serve HTTP/2 on our custom connection.
	// Note: ServeConn will perform the initial HTTP/2 connection preface and then
	// multiplex streams. This requires that the underlying connection support
	// full-duplex communication.
	h2Server.ServeConn(conn, &http2.ServeConnOpts{
		Handler: handler,
	})
}
