# test empty rhs

mkfile -t lhs a/0 a/1 b/1 c/2

sync

expect ld="a b c" lf="a/0 a/1 b/1 c/2"