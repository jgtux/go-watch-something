// Package memstorage implements an in-memory storage.ClientImpl for
// anacrolix/torrent. The library ships file, mmap, bolt, and sqlite
// backends but nothing memory-backed, so this exists to let
// go-watch-something stream without ever writing the video data to disk.
package memstorage

import (
	"context"
	"fmt"
	"sync"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

type client struct {
	completion storage.PieceCompletion

	mu     sync.Mutex
	pieces map[metainfo.PieceKey][]byte
}

// New returns a storage.ClientImpl that keeps all piece data in memory.
func New() storage.ClientImpl {
	return &client{
		completion: storage.NewMapPieceCompletion(),
		pieces:     make(map[metainfo.PieceKey][]byte),
	}
}

func (c *client) OpenTorrent(_ context.Context, info *metainfo.Info, infoHash metainfo.Hash) (storage.TorrentImpl, error) {
	t := &memTorrent{client: c, infoHash: infoHash}
	return storage.TorrentImpl{
		Piece: t.Piece,
		Close: func() error { return nil },
	}, nil
}

type memTorrent struct {
	client   *client
	infoHash metainfo.Hash
}

func (t *memTorrent) Piece(p metainfo.Piece) storage.PieceImpl {
	return &memPiece{
		client: t.client,
		key:    metainfo.PieceKey{InfoHash: t.infoHash, Index: p.Index()},
		length: p.Length(),
	}
}

type memPiece struct {
	client *client
	key    metainfo.PieceKey
	length int64
}

func (p *memPiece) bytes() []byte {
	p.client.mu.Lock()
	defer p.client.mu.Unlock()
	b, ok := p.client.pieces[p.key]
	if !ok {
		b = make([]byte, p.length)
		p.client.pieces[p.key] = b
	}
	return b
}

func (p *memPiece) ReadAt(b []byte, off int64) (int, error) {
	data := p.bytes()
	if off >= int64(len(data)) {
		return 0, fmt.Errorf("memstorage: read offset %d past piece length %d", off, len(data))
	}
	n := copy(b, data[off:])
	return n, nil
}

func (p *memPiece) WriteAt(b []byte, off int64) (int, error) {
	data := p.bytes()
	if off+int64(len(b)) > int64(len(data)) {
		return 0, fmt.Errorf("memstorage: write past piece length %d (offset=%d, len=%d)", len(data), off, len(b))
	}
	n := copy(data[off:], b)
	return n, nil
}

func (p *memPiece) MarkComplete() error {
	return p.client.completion.Set(p.key, true)
}

func (p *memPiece) MarkNotComplete() error {
	return p.client.completion.Set(p.key, false)
}

func (p *memPiece) Completion() storage.Completion {
	c, err := p.client.completion.Get(p.key)
	if err != nil {
		return storage.Completion{Ok: false, Err: err}
	}
	return c
}
