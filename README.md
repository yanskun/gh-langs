# gh-langs
GitHub CLI commands extension.

Outputs the number of lines for each language in the target account.

## Install
```shell
gh extension install yanskun/gh-langs
```

## Usage
```shell
# login user
gh langas
# arg
gh langs octocat
+------------+---------+
| LANGUAGE   | LINES   |
+------------+---------+
| Ruby       | 204,865 |
| CSS        | 14,950  |
| HTML       | 4,338   |
| Shell      | 910     |
| JavaScript | 48      |
+------------+---------+
https:github.com/octocat has 8 repositories
```
