# files only on rhs

mkfile -t rhs a/b/0 a/c/1 a/d/2

sync

expect ro="a a/b a/c a/d a/b/0 a/c/1 a/d/2"
