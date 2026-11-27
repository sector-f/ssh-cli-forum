package sqlite

import "testing"

func TestValidUsernames(t *testing.T) {
	tt := []struct {
		username string
		desc     string
		valid    bool
	}{
		{
			username: "foobar",
			desc:     "Normal username",
			valid:    true,
		},
		{
			username: "foo bar",
			desc:     "Space in the middle is invalid",
			valid:    false,
		},
		{
			username: "",
			desc:     "The empty string is invalid",
			valid:    false,
		},
		{
			username: "    ",
			desc:     "Multiple spaces is invalid",
			valid:    false,
		},
		{
			username: "FOOBAR",
			desc:     "Capital letters are allowed",
			valid:    true,
		},
		{
			username: "foo_bar",
			desc:     "Underscores are allowed",
			valid:    true,
		},
		{
			username: "foo-bar",
			desc:     "Hyphens are allowed",
			valid:    true,
		},
		{
			username: "-foobar_",
			desc:     "Hyphens and underscores are allowed at start and end",
			valid:    true,
		},
		{
			username: "123456789",
			desc:     "Numbers are allowed",
			valid:    true,
		},
		{
			username: "0123456789abcdef0123",
			desc:     "20 character username is allowed",
			valid:    true,
		},
		{
			username: "12",
			desc:     "2 character username is invalid",
			valid:    false,
		},
		{
			username: "123",
			desc:     "3 character username is allowed",
			valid:    true,
		},
		{
			username: "0123456789abcdef01234",
			desc:     "21 character username is invalid",
			valid:    false,
		},
	}

	for _, test := range tt {
		err := isValidUsername(test.username)
		if (test.valid && err == nil) || !test.valid && err != nil {
			continue
		}

		validStr := "valid"
		if !test.valid {
			validStr = "not valid"
		}

		t.Errorf("%q: %q wrongly reported as %s", test.desc, test.username, validStr)
	}
}
