package normalize

import "testing"

func TestToSnakeCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"taskId", "task_id"},
		{"assignedTo", "assigned_to"},
		{"alreadySnake", "already_snake"},
		{"already_snake", "already_snake"},
		{"HTMLParser", "htmlparser"},
		{"id", "id"},
		{"ID", "id"},
		{"profileSlug", "profile_slug"},
		{"parentTaskID", "parent_task_id"},
		{"", ""},
	}
	for _, tc := range cases {
		got := toSnakeCase(tc.in)
		if got != tc.want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestJSONKeys(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"camelCase object",
			`{"taskId":"123","assignedTo":"bot-a","nestedObj":{"parentGoalId":"g1"}}`,
			`{"assigned_to":"bot-a","nested_obj":{"parent_goal_id":"g1"},"task_id":"123"}`,
		},
		{
			"already snake_case",
			`{"task_id":"123","assigned_to":"bot-a"}`,
			`{"assigned_to":"bot-a","task_id":"123"}`,
		},
		{
			"array of objects",
			`[{"taskId":"1"},{"taskId":"2"}]`,
			`[{"task_id":"1"},{"task_id":"2"}]`,
		},
		{
			"not json",
			`hello world`,
			`hello world`,
		},
		{
			"empty string",
			``,
			``,
		},
		{
			"plain text with braces in middle",
			`some text`,
			`some text`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := JSONKeys(tc.in)
			if got != tc.want {
				t.Errorf("JSONKeys(%q) =\n  %q\nwant:\n  %q", tc.in, got, tc.want)
			}
		})
	}
}
