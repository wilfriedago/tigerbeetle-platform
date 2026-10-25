package devstack

import "testing"

func TestLoad(t *testing.T) {
	t.Parallel()
	images, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if images.TigerBeetle == "" || images.Postgres == "" || images.RabbitMQ == "" || images.NATS == "" || images.Kafka == "" {
		t.Fatalf("images = %+v", images)
	}
	t.Logf("images = %+v", images)
}
