package store

import "testing"

func TestRecordUsageTracksDailyAndMonthlyPeriods(t *testing.T) {
	st := openIsolatedStore(t)
	_, err := st.CreateKey("quota", "user", "", 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := st.ListKeys()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordUsage(keys[0].ID, 25); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordUsage(keys[0].ID, 5); err != nil {
		t.Fatal(err)
	}

	allowed, used, limit, err := st.CheckQuota(keys[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || used != 30 || limit != 100 {
		t.Fatalf("CheckQuota() = %v, %d, %d, want true, 30, 100", allowed, used, limit)
	}
	snapshots, err := st.GetQuotaSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots[0].UsedToday != 30 || snapshots[0].UsedThisMonth != 30 || snapshots[0].RemainingToday != 70 || snapshots[0].RemainingMonth != 970 {
		t.Fatalf("quota snapshots = %#v", snapshots)
	}
}

func TestCheckQuotaCurrentBaselineDoesNotEnforceMonthlyLimit(t *testing.T) {
	st := openIsolatedStore(t)
	_, err := st.CreateKey("monthly-baseline", "user", "", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := st.ListKeys()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordUsage(keys[0].ID, 1); err != nil {
		t.Fatal(err)
	}

	allowed, used, dailyLimit, err := st.CheckQuota(keys[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || used != 1 || dailyLimit != 0 {
		t.Fatalf("current monthly baseline = allowed %v, used %d, daily limit %d", allowed, used, dailyLimit)
	}
	t.Log("known gap: CheckQuota ignores monthly_limit; Task 51 owns atomic budget enforcement")
}
