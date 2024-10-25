# test lhs and rhs

# one common file
mkfile -t both a/b/0

# one exclusive file
mkfile -t lhs a/d/1
mkfile -t rhs a/f/1

sync

expect same="a a/b a/b/0" diff="" lo="a/d a/d/1" ro="a/f a/f/1"
