# test lhs and rhs diffs

mkfile -t both a/b/0
mkfile -t lhs a/d/1
mkfile -t rhs a/f/1

expect lo="a/d/1" ro="a/f/1" same="a/b/0"
