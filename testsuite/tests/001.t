# test empty rhs

mkfile -t lhs a/b/0 a/c/1 a/d/2

sync

expect lo="a a/b a/c a/d a/b/0 a/c/1 a/d/2"
