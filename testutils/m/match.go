package m

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/matrix-org/sync-v3/sync3"
)

type RespMatcher func(res *sync3.Response) error
type ListMatcher func(list sync3.ResponseList) error
type OpMatcher func(op sync3.ResponseOp) error
type RoomMatcher func(r sync3.Room) error

func MatchRoomName(name string) RoomMatcher {
	return func(r sync3.Room) error {
		if name == "" {
			return nil
		}
		if r.Name != name {
			return fmt.Errorf("name mismatch, got %s want %s", r.Name, name)
		}
		return nil
	}
}

func MatchJoinCount(count int) RoomMatcher {
	return func(r sync3.Room) error {
		if r.JoinedCount != count {
			return fmt.Errorf("MatchJoinCount: got %v want %v", r.JoinedCount, count)
		}
		return nil
	}
}

func MatchInviteCount(count int) RoomMatcher {
	return func(r sync3.Room) error {
		if r.InvitedCount != count {
			return fmt.Errorf("MatchInviteCount: got %v want %v", r.InvitedCount, count)
		}
		return nil
	}
}

func MatchRoomRequiredState(events []json.RawMessage) RoomMatcher {
	return func(r sync3.Room) error {
		if len(r.RequiredState) != len(events) {
			return fmt.Errorf("required state length mismatch, got %d want %d", len(r.RequiredState), len(events))
		}
		// allow any ordering for required state
		for _, want := range events {
			found := false
			for _, got := range r.RequiredState {
				if bytes.Equal(got, want) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("required state want event %v but it does not exist", string(want))
			}
		}
		return nil
	}
}
func MatchRoomInviteState(events []json.RawMessage) RoomMatcher {
	return func(r sync3.Room) error {
		if len(r.InviteState) != len(events) {
			return fmt.Errorf("invite state length mismatch, got %d want %d", len(r.InviteState), len(events))
		}
		// allow any ordering for required state
		for _, want := range events {
			found := false
			for _, got := range r.InviteState {
				if bytes.Equal(got, want) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("required state want event %v but it does not exist", string(want))
			}
		}
		return nil
	}
}

// Similar to MatchRoomTimeline but takes the last n events of `events` and only checks with the last
// n events of the timeline.
func MatchRoomTimelineMostRecent(n int, events []json.RawMessage) RoomMatcher {
	subset := events[len(events)-n:]
	return func(r sync3.Room) error {
		if len(r.Timeline) < len(subset) {
			return fmt.Errorf("timeline length mismatch: got %d want at least %d", len(r.Timeline), len(subset))
		}
		gotSubset := r.Timeline[len(r.Timeline)-n:]
		for i := range gotSubset {
			if !bytes.Equal(gotSubset[i], subset[i]) {
				return fmt.Errorf("timeline[%d]\ngot  %v \nwant %v", i, string(r.Timeline[i]), string(events[i]))
			}
		}
		return nil
	}
}

func MatchRoomPrevBatch(prevBatch string) RoomMatcher {
	return func(r sync3.Room) error {
		if prevBatch != r.PrevBatch {
			return fmt.Errorf("MatchRoomPrevBatch: got %v want %v", r.PrevBatch, prevBatch)
		}
		return nil
	}
}

// Match the timeline with exactly these events in exactly this order
func MatchRoomTimeline(events []json.RawMessage) RoomMatcher {
	return func(r sync3.Room) error {
		if len(r.Timeline) != len(events) {
			return fmt.Errorf("timeline length mismatch: got %d want %d", len(r.Timeline), len(events))
		}
		for i := range r.Timeline {
			if !bytes.Equal(r.Timeline[i], events[i]) {
				return fmt.Errorf("timeline[%d]\ngot  %v \nwant %v", i, string(r.Timeline[i]), string(events[i]))
			}
		}
		return nil
	}
}
func MatchRoomHighlightCount(count int64) RoomMatcher {
	return func(r sync3.Room) error {
		if r.HighlightCount != count {
			return fmt.Errorf("highlight count mismatch, got %d want %d", r.HighlightCount, count)
		}
		return nil
	}
}
func MatchRoomNotificationCount(count int64) RoomMatcher {
	return func(r sync3.Room) error {
		if r.NotificationCount != count {
			return fmt.Errorf("notification count mismatch, got %d want %d", r.NotificationCount, count)
		}
		return nil
	}
}

func MatchRoomInitial(initial bool) RoomMatcher {
	return func(r sync3.Room) error {
		if r.Initial != initial {
			return fmt.Errorf("MatchRoomInitial: got %v want %v", r.Initial, initial)
		}
		return nil
	}
}

func MatchV3Count(wantCount int) ListMatcher {
	return func(res sync3.ResponseList) error {
		if res.Count != wantCount {
			return fmt.Errorf("list got count %d want %d", res.Count, wantCount)
		}
		return nil
	}
}

func MatchRoomSubscriptionsStrict(wantSubs map[string][]RoomMatcher) RespMatcher {
	return func(res *sync3.Response) error {
		if len(res.Rooms) != len(wantSubs) {
			return fmt.Errorf("MatchRoomSubscriptionsStrict: strict length on: got %v subs want %v", len(res.Rooms), len(wantSubs))
		}
		for roomID, matchers := range wantSubs {
			room, ok := res.Rooms[roomID]
			if !ok {
				return fmt.Errorf("MatchRoomSubscriptionsStrict: want sub for %s but it was missing", roomID)
			}
			for _, m := range matchers {
				if err := m(room); err != nil {
					return fmt.Errorf("MatchRoomSubscriptionsStrict: %s", err)
				}
			}
		}
		return nil
	}
}

func MatchRoomSubscription(roomID string, matchers ...RoomMatcher) RespMatcher {
	return func(res *sync3.Response) error {
		room, ok := res.Rooms[roomID]
		if !ok {
			return fmt.Errorf("MatchRoomSubscription: want sub for %s but it was missing", roomID)
		}
		for _, m := range matchers {
			if err := m(room); err != nil {
				return fmt.Errorf("MatchRoomSubscription: %s", err)
			}
		}
		return nil
	}
}

func MatchRoomSubscriptions(wantSubs map[string][]RoomMatcher) RespMatcher {
	return func(res *sync3.Response) error {
		for roomID, matchers := range wantSubs {
			room, ok := res.Rooms[roomID]
			if !ok {
				return fmt.Errorf("MatchRoomSubscriptions: want sub for %s but it was missing", roomID)
			}
			for _, m := range matchers {
				if err := m(room); err != nil {
					return fmt.Errorf("MatchRoomSubscriptions[%s]: %s", roomID, err)
				}
			}
		}
		return nil
	}
}

func MatchOTKCounts(otkCounts map[string]int) RespMatcher {
	return func(res *sync3.Response) error {
		if res.Extensions.E2EE == nil {
			return fmt.Errorf("MatchOTKCounts: no E2EE extension present")
		}
		if !reflect.DeepEqual(res.Extensions.E2EE.OTKCounts, otkCounts) {
			return fmt.Errorf("MatchOTKCounts: got %v want %v", res.Extensions.E2EE.OTKCounts, otkCounts)
		}
		return nil
	}
}

func MatchFallbackKeyTypes(fallbackKeyTypes []string) RespMatcher {
	return func(res *sync3.Response) error {
		if res.Extensions.E2EE == nil {
			return fmt.Errorf("MatchFallbackKeyTypes: no E2EE extension present")
		}
		if !reflect.DeepEqual(res.Extensions.E2EE.FallbackKeyTypes, fallbackKeyTypes) {
			return fmt.Errorf("MatchFallbackKeyTypes: got %v want %v", res.Extensions.E2EE.FallbackKeyTypes, fallbackKeyTypes)
		}
		return nil
	}
}

func MatchDeviceLists(changed, left []string) RespMatcher {
	return func(res *sync3.Response) error {
		if res.Extensions.E2EE == nil {
			return fmt.Errorf("MatchDeviceLists: no E2EE extension present")
		}
		if res.Extensions.E2EE.DeviceLists == nil {
			return fmt.Errorf("MatchDeviceLists: no device lists present")
		}
		if !reflect.DeepEqual(res.Extensions.E2EE.DeviceLists.Changed, changed) {
			return fmt.Errorf("MatchDeviceLists: got changed: %v want %v", res.Extensions.E2EE.DeviceLists.Changed, changed)
		}
		if !reflect.DeepEqual(res.Extensions.E2EE.DeviceLists.Left, left) {
			return fmt.Errorf("MatchDeviceLists: got left: %v want %v", res.Extensions.E2EE.DeviceLists.Left, left)
		}
		return nil
	}
}

func MatchToDeviceMessages(wantMsgs []json.RawMessage) RespMatcher {
	return func(res *sync3.Response) error {
		if res.Extensions.ToDevice == nil {
			return fmt.Errorf("MatchToDeviceMessages: missing to_device extension")
		}
		if len(res.Extensions.ToDevice.Events) != len(wantMsgs) {
			return fmt.Errorf("MatchToDeviceMessages: got %d events, want %d", len(res.Extensions.ToDevice.Events), len(wantMsgs))
		}
		for i := 0; i < len(wantMsgs); i++ {
			if !reflect.DeepEqual(res.Extensions.ToDevice.Events[i], wantMsgs[i]) {
				return fmt.Errorf("MatchToDeviceMessages[%d]: got %v want %v", i, string(res.Extensions.ToDevice.Events[i]), string(wantMsgs[i]))
			}
		}
		return nil
	}
}

func MatchV3SyncOp(start, end int64, roomIDs []string, anyOrder ...bool) OpMatcher {
	allowAnyOrder := len(anyOrder) > 0 && anyOrder[0]
	return func(op sync3.ResponseOp) error {
		if op.Op() != sync3.OpSync {
			return fmt.Errorf("op: %s != %s", op.Op(), sync3.OpSync)
		}
		oper := op.(*sync3.ResponseOpRange)
		if oper.Range[0] != start {
			return fmt.Errorf("%s: got start %d want %d", sync3.OpSync, oper.Range[0], start)
		}
		if oper.Range[1] != end {
			return fmt.Errorf("%s: got end %d want %d", sync3.OpSync, oper.Range[1], end)
		}
		if allowAnyOrder {
			sort.Strings(oper.RoomIDs)
			sort.Strings(roomIDs)
		}
		if !reflect.DeepEqual(roomIDs, oper.RoomIDs) {
			return fmt.Errorf("%s: got rooms %v want %v", sync3.OpSync, oper.RoomIDs, roomIDs)
		}
		return nil
	}
}

func MatchV3SyncOpFn(fn func(op *sync3.ResponseOpRange) error) OpMatcher {
	return func(op sync3.ResponseOp) error {
		if op.Op() != sync3.OpSync {
			return fmt.Errorf("op: %s != %s", op.Op(), sync3.OpSync)
		}
		oper := op.(*sync3.ResponseOpRange)
		return fn(oper)
	}
}

func MatchV3InsertOp(roomIndex int, roomID string) OpMatcher {
	return func(op sync3.ResponseOp) error {
		if op.Op() != sync3.OpInsert {
			return fmt.Errorf("op: %s != %s", op.Op(), sync3.OpInsert)
		}
		oper := op.(*sync3.ResponseOpSingle)
		if *oper.Index != roomIndex {
			return fmt.Errorf("%s: got index %d want %d", sync3.OpInsert, *oper.Index, roomIndex)
		}
		if oper.RoomID != roomID {
			return fmt.Errorf("%s: got %s want %s", sync3.OpInsert, oper.RoomID, roomID)
		}
		return nil
	}
}

func MatchV3DeleteOp(roomIndex int) OpMatcher {
	return func(op sync3.ResponseOp) error {
		if op.Op() != sync3.OpDelete {
			return fmt.Errorf("op: %s != %s", op.Op(), sync3.OpDelete)
		}
		oper := op.(*sync3.ResponseOpSingle)
		if *oper.Index != roomIndex {
			return fmt.Errorf("%s: got room index %d want %d", sync3.OpDelete, *oper.Index, roomIndex)
		}
		return nil
	}
}

func MatchV3InvalidateOp(start, end int64) OpMatcher {
	return func(op sync3.ResponseOp) error {
		if op.Op() != sync3.OpInvalidate {
			return fmt.Errorf("op: %s != %s", op.Op(), sync3.OpInvalidate)
		}
		oper := op.(*sync3.ResponseOpRange)
		if oper.Range[0] != start {
			return fmt.Errorf("%s: got start %d want %d", sync3.OpInvalidate, oper.Range[0], start)
		}
		if oper.Range[1] != end {
			return fmt.Errorf("%s: got end %d want %d", sync3.OpInvalidate, oper.Range[1], end)
		}
		return nil
	}
}

func MatchNoV3Ops() RespMatcher {
	return func(res *sync3.Response) error {
		for i, l := range res.Lists {
			if len(l.Ops) > 0 {
				return fmt.Errorf("MatchNoV3Ops: list %d got %d ops", i, len(l.Ops))
			}
		}
		return nil
	}
}

func MatchV3Ops(matchOps ...OpMatcher) ListMatcher {
	return func(res sync3.ResponseList) error {
		if len(matchOps) != len(res.Ops) {
			return fmt.Errorf("MatchV3Ops: got %d ops want %d", len(res.Ops), len(matchOps))
		}
		for i := range res.Ops {
			op := res.Ops[i]
			if err := matchOps[i](op); err != nil {
				return fmt.Errorf("MatchV3Ops: op[%d](%s) - %s", i, op.Op(), err)
			}
		}
		return nil
	}
}

func MatchAccountData(globals []json.RawMessage, rooms map[string][]json.RawMessage) RespMatcher {
	return func(res *sync3.Response) error {
		if res.Extensions.AccountData == nil {
			return fmt.Errorf("MatchAccountData: no account_data extension")
		}
		if len(globals) > 0 {
			if err := EqualAnyOrder(res.Extensions.AccountData.Global, globals); err != nil {
				return fmt.Errorf("MatchAccountData[global]: %s", err)
			}
		}
		if len(rooms) > 0 {
			if len(rooms) != len(res.Extensions.AccountData.Rooms) {
				return fmt.Errorf("MatchAccountData: got %d rooms with account data, want %d", len(res.Extensions.AccountData.Rooms), len(rooms))
			}
			for roomID := range rooms {
				gots := res.Extensions.AccountData.Rooms[roomID]
				if gots == nil {
					return fmt.Errorf("MatchAccountData: want room account data for %s but it was missing", roomID)
				}
				if err := EqualAnyOrder(gots, rooms[roomID]); err != nil {
					return fmt.Errorf("MatchAccountData[room]: %s", err)
				}
			}
		}
		return nil
	}
}

func CheckList(i int, res sync3.ResponseList, matchers ...ListMatcher) error {
	for _, m := range matchers {
		if err := m(res); err != nil {
			return fmt.Errorf("MatchList[%d]: %v", i, err)
		}
	}
	return nil
}

func MatchTxnID(txnID string) RespMatcher {
	return func(res *sync3.Response) error {
		if txnID != res.TxnID {
			return fmt.Errorf("MatchTxnID: got %v want %v", res.TxnID, txnID)
		}
		return nil
	}
}

func MatchList(i int, matchers ...ListMatcher) RespMatcher {
	return func(res *sync3.Response) error {
		if i >= len(res.Lists) {
			return fmt.Errorf("MatchSingleList: index %d does not exist, got %d lists", i, len(res.Lists))
		}
		list := res.Lists[i]
		return CheckList(i, list, matchers...)
	}
}

func MatchLists(matchers ...[]ListMatcher) RespMatcher {
	return func(res *sync3.Response) error {
		if len(matchers) != len(res.Lists) {
			return fmt.Errorf("MatchLists: got %d matchers for %d lists", len(matchers), len(res.Lists))
		}
		for i := range matchers {
			if err := CheckList(i, res.Lists[i], matchers[i]...); err != nil {
				return fmt.Errorf("MatchLists[%d]: %v", i, err)
			}
		}
		return nil
	}
}

func MatchResponse(t *testing.T, res *sync3.Response, matchers ...RespMatcher) {
	t.Helper()
	for _, m := range matchers {
		err := m(res)
		if err != nil {
			b, _ := json.Marshal(res)
			t.Errorf("MatchResponse: %s\n%+v", err, string(b))
		}
	}
}

func CheckRoom(r sync3.Room, matchers ...RoomMatcher) error {
	for _, m := range matchers {
		if err := m(r); err != nil {
			return fmt.Errorf("MatchRoom : %s", err)
		}
	}
	return nil
}

func EqualAnyOrder(got, want []json.RawMessage) error {
	if len(got) != len(want) {
		return fmt.Errorf("EqualAnyOrder: got %d, want %d", len(got), len(want))
	}
	sort.Slice(got, func(i, j int) bool {
		return string(got[i]) < string(got[j])
	})
	sort.Slice(want, func(i, j int) bool {
		return string(want[i]) < string(want[j])
	})
	for i := range got {
		if !reflect.DeepEqual(got[i], want[i]) {
			return fmt.Errorf("EqualAnyOrder: [%d] got %v want %v", i, string(got[i]), string(want[i]))
		}
	}
	return nil
}
