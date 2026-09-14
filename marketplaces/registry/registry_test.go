package registry_test

import (
	"context"
	"testing"

	"github.com/LikiPiki/PricePulse/internal/domain"
	"github.com/LikiPiki/PricePulse/marketplaces/registry"
)

type adapter struct{ code string }

func (a adapter) Code() string                                         { return a.code }
func (adapter) GetOffer(context.Context, string) (domain.Offer, error) { return domain.Offer{}, nil }

func TestRegistry(t *testing.T) {
	r, err := registry.New(adapter{"second"}, adapter{"first"})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := r.Codes(), []string{"first", "second"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Codes() = %v, want %v", got, want)
	}
	if _, ok := r.Get("first"); !ok {
		t.Fatal("registered adapter not found")
	}
}

func TestRegistryRejectsDuplicateCode(t *testing.T) {
	if _, err := registry.New(adapter{"same"}, adapter{"same"}); err == nil {
		t.Fatal("New() error = nil, want duplicate code error")
	}
}
