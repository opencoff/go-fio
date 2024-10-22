# test lhs and rhs diffs

mkfile lhs="a/b/0 a/c/1 a/d/2" rhs="a/b/0 a/c/1 a/f/0"

mutate lhs="a/b/0" rhs="a/f/0"

expect lo="a/d/2" ro="a/f/0" same="a/c/1 a/d/2 a/b/0" diff="a/b/0 a/f/0"
