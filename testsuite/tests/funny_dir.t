# funny files on one side

mkfile -t both a/b/0 a/c/0 a/b/1 a/c/1

# and one is a directory
mkfile -t lhs a/d/0

# one is a file
mkfile -t rhs a/d

sync

expect cd="a/b a/c" cf="a/b/0 a/b/1 a/c/0 a/c/1" diff="a" funny="a/d"
