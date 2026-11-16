package characters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCharm(t *testing.T) {
	tests := []struct {
		name          string
		userId        int
		rounds        int
		expireCommand string
		want          *CharmInfo
	}{
		{
			name:          "Normal values",
			userId:        1,
			rounds:        5,
			expireCommand: "emote bows",
			want:          &CharmInfo{UserId: 1, RoundsRemaining: 5, ExpiredCommand: "emote bows"},
		},
		{
			name:          "Permanent charm",
			userId:        2,
			rounds:        CharmPermanent,
			expireCommand: CharmExpiredDespawn,
			want:          &CharmInfo{UserId: 2, RoundsRemaining: CharmPermanent, ExpiredCommand: CharmExpiredDespawn},
		},
		{
			name:          "Empty expire command",
			userId:        3,
			rounds:        10,
			expireCommand: "",
			want:          &CharmInfo{UserId: 3, RoundsRemaining: 10, ExpiredCommand: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCharm(tt.userId, tt.rounds, tt.expireCommand)
			assert.Equal(t, tt.want, got)
		})
	}
}
func TestCharmInfo_Expire(t *testing.T) {
	tests := []struct {
		name     string
		initial  *CharmInfo
		expected int
	}{
		{
			name:     "Expire normal charm",
			initial:  &CharmInfo{UserId: 1, RoundsRemaining: 5, ExpiredCommand: "emote bows"},
			expected: 0,
		},
		{
			name:     "Expire permanent charm",
			initial:  &CharmInfo{UserId: 2, RoundsRemaining: CharmPermanent, ExpiredCommand: CharmExpiredDespawn},
			expected: 0,
		},
		{
			name:     "Expire already expired charm",
			initial:  &CharmInfo{UserId: 3, RoundsRemaining: 0, ExpiredCommand: "emote leaves"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.Expire()
			assert.Equal(t, tt.expected, tt.initial.RoundsRemaining)
		})
	}
}
