package devstack

import "testing"

func TestLoad(t *testing.T) {
	t.Parallel()
	images, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if images.TigerBeetle == "" || images.Postgres == "" {
		t.Fatalf("images = %+v", images)
	}
	t.Logf("tigerbeetle=%s postgres=%s", images.TigerBeetle, images.Postgres)
}
