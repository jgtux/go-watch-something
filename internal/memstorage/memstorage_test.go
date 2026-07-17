package memstorage

import (
	"context"
	"testing"

	"github.com/anacrolix/torrent/metainfo"
)

// A minimal single-piece v1 torrent, just enough for Info.Piece(0) to
// compute a real length without needing a full metainfo file on disk.
func testInfo() *metainfo.Info {
	return &metainfo.Info{
		PieceLength: 16,
		// One piece's worth of a v1 torrent: NumPieces() = len(Pieces)/20,
		// and the storage layer doesn't itself verify these hashes (that's
		// the client's job), so any 20 bytes make Piece(0) resolve.
		Pieces: make([]byte, 20),
		Length: 16,
		Name:   "test",
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	c := New()
	info := testInfo()
	var hash metainfo.Hash
	copy(hash[:], "0123456789abcdef0123")

	tor, err := c.OpenTorrent(context.Background(), info, hash)
	if err != nil {
		t.Fatalf("OpenTorrent: %v", err)
	}
	piece := tor.Piece(info.Piece(0))

	want := []byte("hello world!!!!!") // 16 bytes
	n, err := piece.WriteAt(want, 0)
	if err != nil {
		t.Fatalf("WriteAt: %v", err)
	}
	if n != len(want) {
		t.Fatalf("WriteAt wrote %d bytes, want %d", n, len(want))
	}

	got := make([]byte, len(want))
	n, err = piece.ReadAt(got, 0)
	if err != nil {
		t.Fatalf("ReadAt: %v", err)
	}
	if n != len(want) {
		t.Fatalf("ReadAt read %d bytes, want %d", n, len(want))
	}
	if string(got) != string(want) {
		t.Errorf("ReadAt = %q, want %q", got, want)
	}
}

func TestCompletionTracking(t *testing.T) {
	c := New()
	info := testInfo()
	var hash metainfo.Hash
	copy(hash[:], "aaaaaaaaaaaaaaaaaaaa")

	tor, err := c.OpenTorrent(context.Background(), info, hash)
	if err != nil {
		t.Fatalf("OpenTorrent: %v", err)
	}
	piece := tor.Piece(info.Piece(0))

	comp := piece.Completion()
	if comp.Complete {
		t.Errorf("new piece Completion().Complete = true, want false")
	}

	if err := piece.MarkComplete(); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	comp = piece.Completion()
	if !comp.Complete {
		t.Errorf("after MarkComplete, Completion().Complete = false, want true")
	}
	if !comp.Ok {
		t.Errorf("after MarkComplete, Completion().Ok = false, want true")
	}

	if err := piece.MarkNotComplete(); err != nil {
		t.Fatalf("MarkNotComplete: %v", err)
	}
	comp = piece.Completion()
	if comp.Complete {
		t.Errorf("after MarkNotComplete, Completion().Complete = true, want false")
	}
}

func TestWriteAtBoundsCheck(t *testing.T) {
	c := New()
	info := testInfo()
	var hash metainfo.Hash

	tor, _ := c.OpenTorrent(context.Background(), info, hash)
	piece := tor.Piece(info.Piece(0))

	_, err := piece.WriteAt([]byte("this is way too long for a 16 byte piece"), 0)
	if err == nil {
		t.Errorf("WriteAt past piece length = nil error, want error")
	}
}

func TestTwoTorrentsDoNotShareData(t *testing.T) {
	// Same piece index, different infohash -- must not collide, since
	// metainfo.PieceKey includes InfoHash precisely for this reason.
	c := New()
	info := testInfo()
	var hashA, hashB metainfo.Hash
	copy(hashA[:], "AAAAAAAAAAAAAAAAAAAA")
	copy(hashB[:], "BBBBBBBBBBBBBBBBBBBB")

	torA, _ := c.OpenTorrent(context.Background(), info, hashA)
	torB, _ := c.OpenTorrent(context.Background(), info, hashB)
	pieceA := torA.Piece(info.Piece(0))
	pieceB := torB.Piece(info.Piece(0))

	pieceA.WriteAt([]byte("aaaaaaaaaaaaaaaa"), 0)
	pieceB.WriteAt([]byte("bbbbbbbbbbbbbbbb"), 0)

	gotA := make([]byte, 16)
	pieceA.ReadAt(gotA, 0)
	if string(gotA) != "aaaaaaaaaaaaaaaa" {
		t.Errorf("torrent A's piece data = %q, want all-a (leaked from torrent B?)", gotA)
	}
}
