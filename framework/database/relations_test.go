package database

import (
	"reflect"
	"testing"
)

type collMsg struct {
	ID   int64  `db:"id,primaryKey,autoIncrement"`
	Body string `db:"body"`
}

type collConvo struct {
	ID       int64     `db:"id,primaryKey,autoIncrement"`
	Sent     []collMsg `db:"has_many:messages,foreign_key:sender_id"`
	Received []collMsg `db:"has_many:messages,foreign_key:receiver_id"`
}

// TestEagerLoadDistinctRelationsSameTable is the field-collision regression: a
// model with two has_many relations to the SAME table must load each into its
// own field, not whichever the (table,kind) lookup found first.
func TestEagerLoadDistinctRelationsSameTable(t *testing.T) {
	info := getStructInfo(&collConvo{})

	sent, ok := info.relations["Sent"]
	if !ok {
		t.Fatal("Sent relation not registered")
	}
	received, ok := info.relations["Received"]
	if !ok {
		t.Fatal("Received relation not registered")
	}
	if reflect.DeepEqual(sent.fieldIndex, received.fieldIndex) {
		t.Fatal("the two relations must have distinct field indexes")
	}

	// Assign only into "Received" and verify "Sent" stays empty.
	convo := &collConvo{ID: 1}
	rv := reflect.ValueOf(convo).Elem()
	related := map[any][]reflect.Value{
		int64(1): {reflect.ValueOf(collMsg{ID: 99, Body: "hi"})},
	}
	assignHasMany(rv, false, info, received, related)

	if len(convo.Received) != 1 || convo.Received[0].ID != 99 {
		t.Fatalf("Received should hold the related row, got %+v", convo.Received)
	}
	if len(convo.Sent) != 0 {
		t.Fatalf("Sent must remain empty (no collision), got %+v", convo.Sent)
	}
}
