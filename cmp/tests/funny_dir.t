# funny files on one side

mkfile -t both a/b/0 a/c/0 a/b/1 a/c/1

# and one is a directory
mkfile -t lhs a/d/0

# one is a file
mkfile -t rhs a/d

touch

expect cd="a a/b a/c" lf="a/d/0" cf="a/b/0 a/b/1 a/c/0 a/c/1"  funny="a/d"
