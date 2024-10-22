# funny files on one side

mkfile -t both a/b/0 a/c/0

# and one is a directory
mkfile -t lhs a/d/0

# one is a file
mkfile -d -t rhs a/d/0

# sync all time stamps
sync

expect same="a a/b a/b/0 a/c a/c/0" diff="a/d" funny="a/d/0"
