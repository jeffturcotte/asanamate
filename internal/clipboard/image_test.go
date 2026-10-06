package clipboard

import (
	"errors"
	"testing"
)

func TestDecodeAppleData(t *testing.T) {
	got, err := decodeAppleData([]byte("«data PNGf89504E47»\n"))
	if err != nil || string(got) != "\x89PNG" {
		t.Fatalf("decodeAppleData = %q, %v", got, err)
	}
	if _, err := decodeAppleData([]byte("some text")); !errors.Is(err, ErrNoImage) {
		t.Fatalf("err = %v; want ErrNoImage", err)
	}
}
