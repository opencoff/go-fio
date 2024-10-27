# test lhs and rhs

# one common file
mkfile -t both a/b/0

# one exclusive file
mkfile -t lhs a/d/1
mkfile -t rhs a/f/1

sync

expect cd="a a/b" cf="a/b/0" ld="a/d" lf="a/d/1" rd="a/f" rf="a/f/1"
