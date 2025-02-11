# basic test case

# empty dirs on both sides
mkfile -d -t both a b c

# sync time stamps
touch

# do the sync; there should be no diffs
expect cd="a b c"
