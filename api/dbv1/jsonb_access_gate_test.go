package dbv1

import (
	"fmt"
	"testing"

	"bridgerton.audius.co/config"
	"github.com/stretchr/testify/assert"
)

func TestPurchaseGateMath(t *testing.T) {
	userMap := map[int32]FullUser{}
	for i := 1; i < 10; i++ {
		userMap[int32(i)] = FullUser{
			GetUsersRow: GetUsersRow{
				PayoutWallet: fmt.Sprintf("wallet%d", i),
			},
		}
	}

	checkSplits := func(fullGate *FullPurchaseGate, expected []int64) {
		var sum int64
		for idx, expectedVal := range expected {
			wallet := fmt.Sprintf("wallet%d", idx+1)
			actual := fullGate.Splits[wallet]
			assert.Equal(t, expectedVal, actual, fmt.Sprintf("expected %s split to be %d got %d", wallet, expectedVal, actual))
			sum += actual
		}
		assert.Equal(t, sum, int64(fullGate.Price)*10000)
	}

	// test 1
	{
		gate := PurchaseGate{
			Price: 100,
			Splits: []PurchaseSplit{
				{1, 0.00010},
				{2, 1.00000},
				{3, 10.00000},
				{4, 3.33333},
				{5, 3.33333},
				{6, 3.33333},
				{7, 25.0000},
				{8, 50.0000},
				{9, 4.00000},
			},
		}

		cfg := config.Config{
			NetworkTakeRate: 0.0,
		}

		fullGate := gate.ToFullPurchaseGate(cfg, userMap)

		expected := []int64{
			1,
			10000,
			100000,
			33333,
			33333,
			33333,
			250000,
			500000,
			40000,
		}

		checkSplits(fullGate, expected)
	}

	// test 2
	{
		gate := PurchaseGate{
			Price: 197,
			Splits: []PurchaseSplit{
				{1, 0.00010},
				{2, 1.00000},
				{3, 10.00000},
				{4, 3.333333},
				{5, 3.333333},
				{6, 3.333333},
				{7, 25.00000},
				{8, 50.00000},
				{9, 4.00000},
			},
		}

		cfg := config.Config{
			NetworkTakeRate: 0.0,
		}

		fullGate := gate.ToFullPurchaseGate(cfg, userMap)

		expected := []int64{
			2,
			19700,
			197000,
			65666,
			65666,
			65666,
			492500,
			985000,
			78800,
		}

		checkSplits(fullGate, expected)
	}

	// test 3
	{
		gate := PurchaseGate{
			Price: 100,
			Splits: []PurchaseSplit{
				{1, 33.3333},
				{2, 66.6667},
			},
		}

		cfg := config.Config{
			NetworkTakeRate: 0.0,
		}

		fullGate := gate.ToFullPurchaseGate(cfg, userMap)
		expected := []int64{
			333333,
			666667,
		}

		checkSplits(fullGate, expected)
	}

	// test 4
	{
		gate := PurchaseGate{
			Price: 202,
			Splits: []PurchaseSplit{
				{1, 33.33334},
				{2, 33.33333},
				{3, 33.33333},
			},
		}

		cfg := config.Config{
			NetworkTakeRate: 0.0,
		}

		fullGate := gate.ToFullPurchaseGate(cfg, userMap)
		expected := []int64{
			673334,
			673333,
			673333,
		}

		checkSplits(fullGate, expected)
	}
}
