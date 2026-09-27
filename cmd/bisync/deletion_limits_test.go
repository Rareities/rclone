package bisync

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/require"
)

func TestAggregateDeleteCountBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		deleted1 int
		deleted2 int
		maximum  int64
		wantErr  string
	}{
		{name: "at limit on one path", deleted1: 25, maximum: 25},
		{name: "aggregate at limit across paths", deleted1: 13, deleted2: 12, maximum: 25},
		{name: "aggregate exceeds across paths", deleted1: 13, deleted2: 13, maximum: 25, wantErr: "too many aggregate deletes"},
		{name: "one beyond limit", deleted1: 26, maximum: 25, wantErr: "too many aggregate deletes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := &bisyncRun{opt: &Options{MaxDeleteCount: test.maximum, Force: true}}
			ds1 := &deltaSet{deleted: test.deleted1}
			ds2 := &deltaSet{deleted: test.deleted2}
			err := b.checkDeleteLimits(ds1, ds2)
			if test.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantErr)
			}
		})
	}
}

func TestAggregateDeleteCountCannotBeDisabled(t *testing.T) {
	b := &bisyncRun{opt: &Options{MaxDeleteCount: 0, Force: true}}
	err := b.checkDeleteLimits(&deltaSet{}, &deltaSet{})
	require.ErrorContains(t, err, "must be a positive integer")
}

func TestBisyncDeleteProtectionDefaults(t *testing.T) {
	require.Equal(t, 10, DefaultMaxDelete)
	require.EqualValues(t, 25, DefaultMaxDeleteCount)
}

func TestRCBisyncRejectsNonPositiveAbsoluteDeleteLimit(t *testing.T) {
	for _, value := range []int64{0, -1} {
		_, err := rcBisync(context.Background(), rc.Params{"maxDeleteCount": value})
		require.ErrorContains(t, err, "maxDeleteCount must be a positive integer")
	}
}
