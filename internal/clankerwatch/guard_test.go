package clankerwatch

import "testing"

func TestIsReadOnlySQL(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want bool
	}{
		{name: "select", sql: "select * from users", want: true},
		{name: "with", sql: "with rows as (select 1) select * from rows", want: true},
		{name: "explain", sql: "/* check plan */ explain select * from users", want: true},
		{name: "line comment", sql: "-- why\nselect 1", want: true},
		{name: "update", sql: "update users set admin = 1", want: false},
		{name: "delete", sql: "delete from users", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsReadOnlySQL(tc.sql); got != tc.want {
				t.Fatalf("IsReadOnlySQL() = %v, want %v", got, tc.want)
			}
		})
	}
}
