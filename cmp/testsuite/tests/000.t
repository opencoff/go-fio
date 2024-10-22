# basic test case


mkfile -d -t both a b c

# do the sync; there should be no diffs
expect lo="" ro="" diff="" same="" funny=""
