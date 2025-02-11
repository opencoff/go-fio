# test empty lhs

mkfile -t rhs a/0 b/1 c/2

touch

expect rd="a b c" rf="a/0 b/1 c/2"
