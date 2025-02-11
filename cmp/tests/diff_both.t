# test lhs and rhs diffs

# make some common files
mkfile -t both a/b/0 a/b/1 a/c/0 a/c/1

touch

# modify one file on each side
mutate lhs="a/c/0" rhs="a/c/1"

expect cd="a a/b a/c" cf="a/b/0 a/b/1" diff="a/c/0 a/c/1"
