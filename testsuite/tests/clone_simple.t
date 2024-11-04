# clone simple

# make some common files
mkfile -t both a/b/0 a/b/1 a/c/0 a/c/1
mkfile -t lhs a/b/2 a/b/3 a/b/4 a/c/2 a/c/3 a/c/4
mkfile -t rhs a/x/0 a/x/1

sync

clone
