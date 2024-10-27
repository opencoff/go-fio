# symlink tests

mkfile -t both a/b/0 a/c/0

symlink lhs="a/b/L0@a/b/0" rhs="a/x/L0@a/c/1"

# sync all time stamps
sync

# adding a new dir on the rhs changes the dir-link-count; so
# we expect to see "a" in the different bucket.
expect cf="a/b/0 a/c/0" cd="a/c a/b" rd="a/x" rf="a/x/L0" \
    lf="a/b/L0" diff="a"
