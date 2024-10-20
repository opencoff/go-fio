# basic test case


mkfile -d lhs="a b c" rhs="a b c"

# do the sync; there should be no diffs
expect lo="" ro="" diff="" same="" funny=""
