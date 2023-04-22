package mysql

import (
	"io"
	"net"
	"time"
)

const defaultBufSize = 4096
const maxCachedBufSize = 256 * 1024

type buffer struct {
	buf     []byte // buf is a byte buffer who's length and capacity are equal.
	nc      net.Conn
	idx     int
	length  int
	timeout time.Duration
	dbuf    [2][]byte // dbuf is an array with the two byte slices that back this buffer
	flipcnt uint      // flipccnt is the current buffer counter for double-buffering
}

// newBuffer allocates and returns a new buffer.
func newBuffer(nc net.Conn) buffer {
	fg := make([]byte, defaultBufSize)
	return buffer{
		buf:  fg,
		nc:   nc,
		dbuf: [2][]byte{fg, nil},
	}
}

func (b *buffer) flip() {
	b.flipcnt += 1
}

// fill reads into the buffer until at least _need_ bytes are in it
func (b *buffer) fill(need int) error {
	n := b.length

	dest := b.dbuf[b.flipcnt&1]

	// grow buffer if necessary to fit the whole packet.
	if need > len(dest) {
		// Round up to the next multiple of the default size
		dest = make([]byte, ((need/defaultBufSize)+1)*defaultBufSize)


		if len(dest) <= maxCachedBufSize {
			b.dbuf[b.flipcnt&1] = dest
		}
	}

	if n > 0 {
		copy(dest[:n], b.buf[b.idx:])
	}

	b.buf = dest
	b.idx = 0

	for {
		if b.timeout > 0 {
			if err := b.nc.SetReadDeadline(time.Now().Add(b.timeout)); err != nil {
				return err
			}
		}

		nn, err := b.nc.Read(b.buf[n:])
		n += nn

		switch err {
		case nil:
			if n < need {
				continue
			}
			b.length = n
			return nil

		case io.EOF:
			if n >= need {
				b.length = n
				return nil
			}
			return io.ErrUnexpectedEOF

		default:
			return err
		}
	}
}


func (b *buffer) readNext(need int) ([]byte, error) {
	if b.length < need {
		// refill
		if err := b.fill(need); err != nil {
			return nil, err
		}
	}

	offset := b.idx
	b.idx += need
	b.length -= need
	return b.buf[offset:b.idx], nil
}

func (b *buffer) takeBuffer(length int) ([]byte, error) {
	if b.length > 0 {
		return nil, ErrBusyBuffer
	}

	// test (cheap) general case first
	if length <= cap(b.buf) {
		return b.buf[:length], nil
	}

	if length < maxPacketSize {
		b.buf = make([]byte, length)
		return b.buf, nil
	}

	// buffer is larger than we want to store.
	return make([]byte, length), nil
}

func (b *buffer) takeSmallBuffer(length int) ([]byte, error) {
	if b.length > 0 {
		return nil, ErrBusyBuffer
	}
	return b.buf[:length], nil
}


func (b *buffer) takeCompleteBuffer() ([]byte, error) {
	if b.length > 0 {
		return nil, ErrBusyBuffer
	}
	return b.buf, nil
}

// store stores buf, an updated buffer, if its suitable to do so.
func (b *buffer) store(buf []byte) error {
	if b.length > 0 {
		return ErrBusyBuffer
	} else if cap(buf) <= maxPacketSize && cap(buf) > cap(b.buf) {
		b.buf = buf[:cap(buf)]
	}
	return nil
}
