package utils

import (
	"testing"
)

func TestCalculateSplitsValidate(t *testing.T) {
	// Test case 1: price = 100
	price1 := 100
	splits1 := []Split{
		{UserID: intPtr(1), Percentage: 0.00010},
		{UserID: intPtr(2), Percentage: 1.00000},
		{UserID: intPtr(3), Percentage: 10.00000},
		{UserID: intPtr(4), Percentage: 3.33333},
		{UserID: intPtr(5), Percentage: 3.33333},
		{UserID: intPtr(6), Percentage: 3.33333},
		{UserID: intPtr(7), Percentage: 25.0000},
		{UserID: intPtr(8), Percentage: 50.0000},
		{UserID: intPtr(9), Percentage: 4.00000},
	}
	networkTakeRate := 0.0

	resSplits1, err := CalculateSplits(&price1, splits1, &networkTakeRate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	splitMapByUser1 := make(map[int]ExtendedSplit)
	for _, split := range resSplits1 {
		if split.UserID != nil {
			splitMapByUser1[*split.UserID] = split
		}
	}

	// Assertions for test case 1
	if splitMapByUser1[1].Amount != 1 {
		t.Errorf("expected user 1 amount 1, got %d", splitMapByUser1[1].Amount)
	}
	if splitMapByUser1[2].Amount != 10000 {
		t.Errorf("expected user 2 amount 10000, got %d", splitMapByUser1[2].Amount)
	}
	if splitMapByUser1[3].Amount != 100000 {
		t.Errorf("expected user 3 amount 100000, got %d", splitMapByUser1[3].Amount)
	}
	if splitMapByUser1[4].Amount != 33333 {
		t.Errorf("expected user 4 amount 33333, got %d", splitMapByUser1[4].Amount)
	}
	if splitMapByUser1[5].Amount != 33333 {
		t.Errorf("expected user 5 amount 33333, got %d", splitMapByUser1[5].Amount)
	}
	if splitMapByUser1[6].Amount != 33333 {
		t.Errorf("expected user 6 amount 33333, got %d", splitMapByUser1[6].Amount)
	}
	if splitMapByUser1[7].Amount != 250000 {
		t.Errorf("expected user 7 amount 250000, got %d", splitMapByUser1[7].Amount)
	}
	if splitMapByUser1[8].Amount != 500000 {
		t.Errorf("expected user 8 amount 500000, got %d", splitMapByUser1[8].Amount)
	}
	if splitMapByUser1[9].Amount != 40000 {
		t.Errorf("expected user 9 amount 40000, got %d", splitMapByUser1[9].Amount)
	}

	// Test case 2: price = 197
	price2 := 197
	splits2 := []Split{
		{UserID: intPtr(1), Percentage: 0.00010},
		{UserID: intPtr(2), Percentage: 1.00000},
		{UserID: intPtr(3), Percentage: 10.00000},
		{UserID: intPtr(4), Percentage: 3.333333},
		{UserID: intPtr(5), Percentage: 3.333333},
		{UserID: intPtr(6), Percentage: 3.333333},
		{UserID: intPtr(7), Percentage: 25.00000},
		{UserID: intPtr(8), Percentage: 50.00000},
		{UserID: intPtr(9), Percentage: 4.00000},
	}

	resSplits2, err := CalculateSplits(&price2, splits2, &networkTakeRate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	splitMapByUser2 := make(map[int]ExtendedSplit)
	for _, split := range resSplits2 {
		if split.UserID != nil {
			splitMapByUser2[*split.UserID] = split
		}
	}

	// Assertions for test case 2
	if splitMapByUser2[1].Amount != 2 {
		t.Errorf("expected user 1 amount 2, got %d", splitMapByUser2[1].Amount)
	}
	if splitMapByUser2[2].Amount != 19700 {
		t.Errorf("expected user 2 amount 19700, got %d", splitMapByUser2[2].Amount)
	}
	if splitMapByUser2[3].Amount != 197000 {
		t.Errorf("expected user 3 amount 197000, got %d", splitMapByUser2[3].Amount)
	}
	if splitMapByUser2[4].Amount != 65666 {
		t.Errorf("expected user 4 amount 65666, got %d", splitMapByUser2[4].Amount)
	}
	if splitMapByUser2[5].Amount != 65666 {
		t.Errorf("expected user 5 amount 65666, got %d", splitMapByUser2[5].Amount)
	}
	if splitMapByUser2[6].Amount != 65666 {
		t.Errorf("expected user 6 amount 65666, got %d", splitMapByUser2[6].Amount)
	}
	if splitMapByUser2[7].Amount != 492500 {
		t.Errorf("expected user 7 amount 492500, got %d", splitMapByUser2[7].Amount)
	}
	if splitMapByUser2[8].Amount != 985000 {
		t.Errorf("expected user 8 amount 985000, got %d", splitMapByUser2[8].Amount)
	}
	if splitMapByUser2[9].Amount != 78800 {
		t.Errorf("expected user 9 amount 78800, got %d", splitMapByUser2[9].Amount)
	}

	// Test case 3: price = 100, simple two-way split
	price3 := 100
	splits3 := []Split{
		{UserID: intPtr(1), Percentage: 33.3333},
		{UserID: intPtr(2), Percentage: 66.6667},
	}

	resSplits3, err := CalculateSplits(&price3, splits3, &networkTakeRate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	splitMapByUser3 := make(map[int]ExtendedSplit)
	for _, split := range resSplits3 {
		if split.UserID != nil {
			splitMapByUser3[*split.UserID] = split
		}
	}

	// Assertions for test case 3
	if splitMapByUser3[1].Amount != 333333 {
		t.Errorf("expected user 1 amount 333333, got %d", splitMapByUser3[1].Amount)
	}
	if splitMapByUser3[2].Amount != 666667 {
		t.Errorf("expected user 2 amount 666667, got %d", splitMapByUser3[2].Amount)
	}

	// Test case 4: price = 202, three-way split with rounding
	price4 := 202
	splits4 := []Split{
		{UserID: intPtr(1), Percentage: 33.33334},
		{UserID: intPtr(2), Percentage: 33.33333},
		{UserID: intPtr(3), Percentage: 33.33333},
	}

	resSplits4, err := CalculateSplits(&price4, splits4, &networkTakeRate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	splitMapByUser4 := make(map[int]ExtendedSplit)
	for _, split := range resSplits4 {
		if split.UserID != nil {
			splitMapByUser4[*split.UserID] = split
		}
	}

	// Assertions for test case 4
	if splitMapByUser4[1].Amount != 673334 {
		t.Errorf("expected user 1 amount 673334, got %d", splitMapByUser4[1].Amount)
	}
	if splitMapByUser4[2].Amount != 673333 {
		t.Errorf("expected user 2 amount 673333, got %d", splitMapByUser4[2].Amount)
	}
	if splitMapByUser4[3].Amount != 673333 {
		t.Errorf("expected user 3 amount 673333, got %d", splitMapByUser4[3].Amount)
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}
