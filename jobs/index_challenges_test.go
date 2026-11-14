package jobs

import (
	"api.audius.co/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOnlyWeeklyTrendingProcessorsAreScheduled(t *testing.T) {
	job := NewIndexChallengesJob(config.Config{LogLevel: "fatal"}, nil)
	require.Len(t, job.processors, 2)
	require.Equal(t, "tt", job.processors[0].ChallengeID())
	require.Equal(t, "tut", job.processors[1].ChallengeID())
}
