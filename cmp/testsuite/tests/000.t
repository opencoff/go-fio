# basic test case

# empty dirs on both sides
mkfile -d -t both a b c

# sync time stamps
sync

# do the sync; there should be no diffs
expect lo="" ro="" diff="" same="a b c" funny=""
