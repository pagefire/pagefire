package oncall

import (
	"context"
	"math"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

// Resolver computes who is on-call at a given time. On-call is NOT stored as
// state — it's calculated from rotation definitions (same approach as GoAlert).
type Resolver struct {
	schedules store.ScheduleStore
	users     store.UserStore
}

func NewResolver(schedules store.ScheduleStore, users store.UserStore) *Resolver {
	return &Resolver{schedules: schedules, users: users}
}

// Resolve returns the set of users on-call for the given schedule at the given time.
func (r *Resolver) Resolve(ctx context.Context, scheduleID string, at time.Time) ([]store.User, error) {
	rotations, err := r.schedules.ListRotations(ctx, scheduleID)
	if err != nil {
		return nil, err
	}

	// Collect on-call user IDs from all rotations
	onCallIDs := make(map[string]bool)

	for _, rot := range rotations {
		if at.Before(rot.StartTime) {
			continue
		}

		participants, err := r.schedules.ListParticipants(ctx, rot.ID)
		if err != nil {
			return nil, err
		}
		if len(participants) == 0 {
			continue
		}

		duration := shiftDuration(rot)
		if duration <= 0 {
			continue
		}

		elapsed := at.Sub(rot.StartTime)
		index := int(math.Floor(float64(elapsed)/float64(duration))) % len(participants)
		onCallIDs[participants[index].UserID] = true
	}

	// Apply overrides
	overrides, err := r.schedules.ListActiveOverrides(ctx, scheduleID, at)
	if err != nil {
		return nil, err
	}
	for _, o := range overrides {
		if onCallIDs[o.ReplaceUser] {
			delete(onCallIDs, o.ReplaceUser)
			onCallIDs[o.OverrideUser] = true
		}
	}

	// Fetch user objects
	var users []store.User
	for userID := range onCallIDs {
		u, err := r.users.Get(ctx, userID)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}

	return users, nil
}

func shiftDuration(r store.Rotation) time.Duration {
	switch r.Type {
	case store.RotationTypeDaily:
		return time.Duration(r.ShiftLength) * 24 * time.Hour
	case store.RotationTypeWeekly:
		return time.Duration(r.ShiftLength) * 7 * 24 * time.Hour
	case store.RotationTypeCustom:
		return time.Duration(r.ShiftLength) * time.Hour
	default:
		return 0
	}
}
