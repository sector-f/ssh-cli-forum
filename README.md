# SSH Community Forum

A small forum reached over SSH, built with Go, Wish, Bubble Tea, and SQLite.

## Running the server

The SSH server defaults to `127.0.0.1:2222` and uses `./forum.db`.

```sh
go run .
```

Users authenticate with SSH public keys. New accounts are created through the `signup` SSH username and require an invitation code. The signup flow and single-use invite behavior are retained.

## First-time setup

Create the database and schema, then create the first administrator account:

```sh
go run ./cmd/forumctl --db ./forum.db db migrate
go run ./cmd/forumctl --db ./forum.db admin bootstrap \
  --username admin \
  --public-key-file ~/.ssh/id_ed25519.pub
```

Create boards and invitations:

```sh
go run ./cmd/forumctl --db ./forum.db board create general "General Discussion" "General topics"
go run ./cmd/forumctl --db ./forum.db board create programming "Programming" "Software and systems"
go run ./cmd/forumctl --db ./forum.db invite create --expires-in 168h
```

The generated invitation code is printed once. An administrator can also create invitations from an SSH session with `invite create`.

## SSH forum commands

The main interface is command-oriented and does not use Bubble Tea's alternate screen buffer. The `chat` command opens a separate full-screen global chat view in the alternate screen buffer; `/leave` returns to the forum interface.

- `ls`, `cd`, and `pwd` navigate the virtual forum tree.
- `/boards` lists boards; `/boards/<slug>` lists a board's topics.
- `/boards/<slug>/<topic-id>` displays a topic and its replies.
- `new` creates a topic from inside a board.
- `reply` adds a reply from inside a topic.
- `recent` lists recently active topics.
- `chat` opens the persistent global chat. In chat, enter a message to post, use `/me <action>` for an action message, and `/leave` to return to forum navigation. The most recent 100 messages are loaded on entry; the mouse wheel scrolls history.
- `help` lists available commands and permissions.

Multi-line post bodies end with a line containing a single period. Enter `/cancel` to abort. Enter `..` to include a literal period on its own line.

Members can edit and delete their own content. Moderators can lock, hide, restore, pin, unpin, and remove topics or replies. Administrators can manage boards, invitations, account roles, and account status.

There is intentionally no `members` or `search` command in this initial version.

## forumctl

`forumctl` manages the database directly and does not require an SSH login:

```text
forumctl --db PATH db migrate
forumctl --db PATH db status
forumctl --db PATH db backup ./backup.db

forumctl --db PATH admin bootstrap --username NAME --public-key-file KEY

forumctl --db PATH board create SLUG NAME [DESCRIPTION]
forumctl --db PATH board list
forumctl --db PATH board update SLUG NAME DESCRIPTION
forumctl --db PATH board archive SLUG
forumctl --db PATH board restore SLUG

forumctl --db PATH invite create [--expires-in 168h] [--admin]
forumctl --db PATH invite list
forumctl --db PATH invite revoke CODE

forumctl --db PATH user list
forumctl --db PATH user set-role USERNAME member|moderator|admin
forumctl --db PATH user suspend USERNAME
forumctl --db PATH user activate USERNAME
forumctl --db PATH modlog
```

Opening the database applies any pending schema upgrades. Existing databases from the earlier prototype are upgraded in place; users, SSH keys, invitations, boards, topics, and replies are preserved. Backups use SQLite's online `VACUUM INTO` operation.
