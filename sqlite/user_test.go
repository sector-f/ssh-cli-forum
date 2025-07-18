package sqlite

import "testing"

func TestValidUsernames(t *testing.T) {
	tt := []struct {
		username string
		desc     string
		expected bool
	}{
		{
			username: "foobar",
			desc:     "Normal username",
			expected: true,
		},
		{
			username: "foo bar",
			desc:     "Space in the middle is invalid",
			expected: false,
		},
		{
			username: "",
			desc:     "The empty string is invalid",
			expected: false,
		},
		{
			username: "    ",
			desc:     "Multiple spaces is invalid",
			expected: false,
		},
		{
			username: "FOOBAR",
			desc:     "Capital letters are allowed",
			expected: true,
		},
		{
			username: "foo_bar",
			desc:     "Underscores are allowed",
			expected: true,
		},
		{
			username: "foo-bar",
			desc:     "Hyphens are allowed",
			expected: true,
		},
		{
			username: "-foobar_",
			desc:     "Hyphens and underscores are allowed at start and end",
			expected: true,
		},
		{
			username: "123456789",
			desc:     "Numbers are allowed",
			expected: true,
		},
		{
			username: "0123456789abcdef0123",
			desc:     "20 character username is allowed",
			expected: true,
		},
		{
			username: "0123456789abcdef01234",
			desc:     "21 character username is invalid",
			expected: false,
		},
	}

	for _, test := range tt {
		actual := isValidUsername(test.username)
		if actual != test.expected {
			t.Errorf("%q: expected %v, got %v", test.desc, test.expected, actual)
		}
	}
}
