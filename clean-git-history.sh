#!/bin/bash
set -e

echo "Creating backup branch: backup-before-filter"
git branch -f backup-before-filter

echo "Removing large files from Git history..."
git filter-branch --force --index-filter \
  'git rm --cached --ignore-unmatch \
     graph-data/* \
     results.json \
     results.txt \
     results-dedup.txt \
     results*.txt \
     results*.json \
     bin/dgraph-server \
     dgraph-server \
     *.jsonl \
     *.db \
     *.sqlite \
     *.sqlite3 \
     *.dat \
     *.bin' \
  --prune-empty --tag-name-filter cat -- --all

echo "Cleaning up refs..."
git for-each-ref --format="delete %(refname)" refs/original/ | git update-ref --stdin
git reflog expire --expire=now --all
git gc --prune=now --aggressive

echo "Git history has been cleaned."
echo "To push to GitHub, you may need to force push:"
echo "git push origin --force --all"
echo "git push origin --force --tags"
echo ""
echo "IMPORTANT: This will overwrite history on GitHub. Make sure your teammates are aware of this change."
