# funny files on one side

mkfile -t both a/b/0 a/c/0

# and one is a directory
mkfile -t lhs a/d/0

# one is a file
mkfile -d -t rhs a/d/0

# sync all time stamps
touch

expect cd="a a/b a/c a/d" cf="a/b/0 a/c/0" funny="a/d/0"
