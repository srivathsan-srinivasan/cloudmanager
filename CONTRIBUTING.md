# Contributing to CloudManager

First off, thank you for considering contributing to CloudManager. It's people like you that make CloudManager such a great tool.

### 1. Where do I go from here?

If you've noticed a bug or have a feature request, make sure to check our [Issues](https://github.com/yourusername/cloudmanager/issues) page to see if someone else has already created a ticket. If not, go ahead and [create one](https://github.com/yourusername/cloudmanager/issues/new)!

### 2. Fork & create a branch

If this is something you think you can fix, then [fork CloudManager](https://github.com/yourusername/cloudmanager/fork) and create a branch with a descriptive name.

```bash
git checkout -b fix-aws-parser
```

### 3. Get the test suite running

Currently, we are in the process of building out our test suite. Ensure your code compiles locally and run standard go tools:

```bash
go build -o cloudmanager
go fmt ./...
go vet ./...
```

Make sure any new files or features added have proper comments and conform to standard Go idioms.

### 4. Implement your fix or feature

At this point, you're ready to make your changes. Please review the [roadmap](tasks.md) before undertaking major structural changes. We are actively working through phases and want to avoid duplicating effort.

### 5. Make a Pull Request

At this point, you should switch back to your master branch and make sure it's up to date with CloudManager's master branch:

```bash
git remote add upstream git@github.com:yourusername/cloudmanager.git
git checkout master
git pull upstream master
```

Then update your feature branch from your local copy of master, and push it!

```bash
git checkout fix-aws-parser
git rebase master
git push --set-upstream origin fix-aws-parser
```

Finally, go to GitHub and [make a Pull Request](https://github.com/yourusername/cloudmanager/compare) :D
