//go:build linux

package splice

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"

	"benchmark-splice/backend"
)

const maxHeaderBytes = 32 * 1024
const spliceFMove = 1

func Supported() bool {
	return true
}

func serve(ctx context.Context, w http.ResponseWriter, spec backend.Spec) error {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("response writer does not support hijacking")
	}

	downstream, rw, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	defer downstream.Close()

	if err := writeResponseHeader(rw, spec.TotalBytes); err != nil {
		return err
	}

	target, err := parseTarget(spec.UpstreamURL)
	if err != nil {
		return err
	}

	for i := 0; i < spec.Requests(); i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := spec.BytesForChunk(i)
		if n <= 0 {
			continue
		}
		if err := spliceOne(ctx, downstream, target, n); err != nil {
			return err
		}
	}
	return nil
}

func writeResponseHeader(rw *bufio.ReadWriter, totalBytes int64) error {
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", totalBytes); err != nil {
		return err
	}
	return rw.Flush()
}

type rawTarget struct {
	addr string
	host string
	path string
}

func parseTarget(raw string) (rawTarget, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return rawTarget{}, err
	}
	if u.Scheme != "http" {
		return rawTarget{}, fmt.Errorf("splice backend only supports http upstreams, got %q", u.Scheme)
	}
	host := u.Host
	addr := host
	if !strings.Contains(host, ":") {
		addr = net.JoinHostPort(host, "80")
	}
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	return rawTarget{addr: addr, host: host, path: path}, nil
}

func spliceOne(ctx context.Context, downstream net.Conn, target rawTarget, n int64) error {
	dialer := net.Dialer{}
	upstream, err := dialer.DialContext(ctx, "tcp", target.addr)
	if err != nil {
		return err
	}
	defer upstream.Close()

	if _, err := fmt.Fprintf(upstream, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target.path, target.host); err != nil {
		return err
	}
	header, err := readHeader(upstream)
	if err != nil {
		return err
	}
	if err := validateHeader(header); err != nil {
		return err
	}

	src, err := tcpFile(upstream, "upstream")
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := tcpFile(downstream, "downstream")
	if err != nil {
		return err
	}
	defer dst.Close()

	return spliceN(int(src.Fd()), int(dst.Fd()), n)
}

func readHeader(conn net.Conn) ([]byte, error) {
	header := make([]byte, 0, 512)
	var one [1]byte
	for len(header) < maxHeaderBytes {
		n, err := conn.Read(one[:])
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		header = append(header, one[0])
		if len(header) >= 4 && bytes.Equal(header[len(header)-4:], []byte("\r\n\r\n")) {
			return header, nil
		}
	}
	return nil, fmt.Errorf("upstream response header exceeded %d bytes", maxHeaderBytes)
}

func validateHeader(header []byte) error {
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(header)), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}
	if resp.ContentLength == 0 {
		return fmt.Errorf("upstream returned empty chunk")
	}
	return nil
}

func tcpFile(conn net.Conn, label string) (*os.File, error) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, fmt.Errorf("%s connection is %T, want *net.TCPConn", label, conn)
	}
	return tcp.File()
}

func spliceN(srcFD, dstFD int, n int64) error {
	var pipe [2]int
	if err := syscall.Pipe(pipe[:]); err != nil {
		return err
	}
	defer syscall.Close(pipe[0])
	defer syscall.Close(pipe[1])

	remaining := n
	for remaining > 0 {
		step := int(remaining)
		if step > 1<<20 {
			step = 1 << 20
		}

		in, err := spliceRetry(srcFD, pipe[1], step)
		if err != nil {
			return err
		}
		if in == 0 {
			return fmt.Errorf("upstream ended with %s bytes remaining", strconv.FormatInt(remaining, 10))
		}

		written := 0
		for written < in {
			out, err := spliceRetry(pipe[0], dstFD, in-written)
			if err != nil {
				return err
			}
			if out == 0 {
				return fmt.Errorf("downstream accepted 0 bytes")
			}
			written += out
		}
		remaining -= int64(in)
	}
	return nil
}

func spliceRetry(srcFD, dstFD, n int) (int, error) {
	for {
		m, _, errno := syscall.Syscall6(syscall.SYS_SPLICE, uintptr(srcFD), 0, uintptr(dstFD), 0, uintptr(n), spliceFMove)
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			continue
		}
		if errno != 0 {
			return 0, errno
		}
		return int(m), nil
	}
}
