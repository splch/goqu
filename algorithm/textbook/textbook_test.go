package textbook_test

import (
	"context"
	"testing"

	"github.com/splch/goqu/algorithm/textbook"
)

// Bernstein-Vazirani tests

func TestBV_Secret5_3Qubits(t *testing.T) {
	cfg := textbook.BVConfig{
		Secret:    5,
		NumQubits: 3,
		Shots:     2048,
	}
	res, err := textbook.BernsteinVazirani(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Secret != 5 {
		t.Errorf("recovered secret = %d, want 5", res.Secret)
	}
}

func TestBV_Secret0_2Qubits(t *testing.T) {
	cfg := textbook.BVConfig{
		Secret:    0,
		NumQubits: 2,
		Shots:     1024,
	}
	res, err := textbook.BernsteinVazirani(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Secret != 0 {
		t.Errorf("recovered secret = %d, want 0", res.Secret)
	}
}

func TestBV_Errors(t *testing.T) {
	_, err := textbook.BernsteinVazirani(context.Background(), textbook.BVConfig{
		NumQubits: 0,
		Secret:    0,
	})
	if err == nil {
		t.Error("expected error for 0 qubits")
	}

	_, err = textbook.BernsteinVazirani(context.Background(), textbook.BVConfig{
		NumQubits: 2,
		Secret:    4, // out of range for 2 qubits
	})
	if err == nil {
		t.Error("expected error for secret out of range")
	}
}

// Deutsch-Jozsa tests

func TestDJ_Constant0(t *testing.T) {
	cfg := textbook.DJConfig{
		NumQubits: 3,
		Oracle:    textbook.ConstantOracle(0),
		Shots:     1024,
	}
	res, err := textbook.DeutschJozsa(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsConstant {
		t.Error("expected IsConstant = true for constant-0 oracle")
	}
}

func TestDJ_Constant1(t *testing.T) {
	cfg := textbook.DJConfig{
		NumQubits: 3,
		Oracle:    textbook.ConstantOracle(1),
		Shots:     1024,
	}
	res, err := textbook.DeutschJozsa(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsConstant {
		t.Error("expected IsConstant = true for constant-1 oracle")
	}
}

func TestDJ_Balanced(t *testing.T) {
	cfg := textbook.DJConfig{
		NumQubits: 3,
		Oracle:    textbook.BalancedOracle(5), // mask = 101
		Shots:     1024,
	}
	res, err := textbook.DeutschJozsa(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsConstant {
		t.Error("expected IsConstant = false for balanced oracle")
	}
}

func TestDJ_Errors(t *testing.T) {
	_, err := textbook.DeutschJozsa(context.Background(), textbook.DJConfig{
		NumQubits: 0,
		Oracle:    textbook.ConstantOracle(0),
	})
	if err == nil {
		t.Error("expected error for 0 qubits")
	}

	_, err = textbook.DeutschJozsa(context.Background(), textbook.DJConfig{
		NumQubits: 2,
		Oracle:    nil,
	})
	if err == nil {
		t.Error("expected error for nil oracle")
	}
}

// Simon's algorithm tests

func TestSimon_Period3_2Qubits(t *testing.T) {
	cfg := textbook.SimonConfig{
		NumQubits: 2,
		Oracle:    textbook.TwoToOneOracle(3, 2),
		Shots:     2048,
	}
	res, err := textbook.Simon(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Period != 3 {
		t.Errorf("recovered period = %d, want 3", res.Period)
	}
}

func TestSimon_Errors(t *testing.T) {
	_, err := textbook.Simon(context.Background(), textbook.SimonConfig{
		NumQubits: 0,
		Oracle:    textbook.TwoToOneOracle(0, 0),
	})
	if err == nil {
		t.Error("expected error for 0 qubits")
	}

	_, err = textbook.Simon(context.Background(), textbook.SimonConfig{
		NumQubits: 2,
		Oracle:    nil,
	})
	if err == nil {
		t.Error("expected error for nil oracle")
	}
}
