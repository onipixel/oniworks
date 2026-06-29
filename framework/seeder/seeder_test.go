package seeder

import (
	"context"
	"testing"

	"github.com/onipixel/oniworks/framework/database"
)

// sampleSeeder mirrors the shape produced by stubs/seeder.stub. The compile-time
// assertion below guarantees a generated seeder satisfies the Seeder interface —
// the regression the seeder.stub signature fix targets.
type sampleSeeder struct{}

func (s *sampleSeeder) Run(ctx context.Context, db *database.DB) error { return nil }

var _ Seeder = (*sampleSeeder)(nil)

// TestRegisterAndShape is a light runtime check that a generated-shape seeder
// registers cleanly.
func TestRegisterAndShape(t *testing.T) {
	r := New()
	r.Register("SampleSeeder", &sampleSeeder{})
	if len(r.seeders) != 1 {
		t.Fatalf("expected 1 registered seeder, got %d", len(r.seeders))
	}
}
