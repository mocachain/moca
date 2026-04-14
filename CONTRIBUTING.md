# Moca Contributor Guidelines

<!-- markdown-link-check-disable -->
- [Moca Contributor Guidelines](#moca-contributor-guidelines)
    - [General Procedure](#general-procedure)
    - [Architecture Decision Records (ADR)](#architecture-decision-records-adr)
    - [Forking](#forking)
    - [Dependencies](#dependencies)
    - [Protobuf](#protobuf)
    - [Development Procedure](#development-procedure)
    - [Testing](#testing)
    - [Updating Documentation](#updating-documentation)
    - [Branching Model and Release](#branching-model-and-release)
        - [Commit messages](#commit-messages)
        - [PR Targeting](#pr-targeting)
        - [Pull Requests](#pull-requests)
        - [Process for reviewing PRs](#process-for-reviewing-prs)
        - [Pull Merge Procedure](#pull-merge-procedure)
        - [Release Procedure](#release-procedure)
<!-- markdown-link-check-enable -->

## <span id="general_procedure">General Procedure</span>

Thank you for considering making contributions to Moca and related repositories.

Moca follows [Tendermint's coding repo](https://github.com/tendermint/coding)
for overall information on repository workflow and standards.

Contributing to this repo can mean many things such as participating in discussion or proposing code changes.
To ensure a smooth workflow for all contributors,
the following general procedure for contributing has been established:

1. Either [open](https://github.com/mocachain/moca/issues/new/choose)
   or [find](https://github.com/mocachain/moca/issues) an issue you have identified and would like to contribute to
   resolving.
2. Participate in thoughtful discussion on that issue.
3. If you would like to contribute:
    1. If the issue is a proposal, ensure that the proposal has been accepted by the Moca maintainers.
    2. Ensure that nobody else has already begun working on the same issue. If someone already has, please make sure to
       contact the individual to collaborate.
    3. If nobody has been assigned the issue and you would like to work on it,
       make a comment on the issue to inform the community of your intention to begin work.
       Ideally, wait for confirmation that no one has started it.
       However, if you do not get a prompt response, feel free to proceed.
    4. Follow standard GitHub best practices:
        1. Fork the repo
        2. Branch from the HEAD of `main`
        3. Make commits
        4. Submit a PR to `main`
    5. Submit the PR in `Draft` mode when the work is still in progress.
       Submit your PR early, even if it is incomplete, so the community can provide comments early in the development
       process.
    6. When the code is complete it can be marked `Ready for Review`.
    7. Include a relevant changelog entry in the `Unreleased` section of `CHANGELOG.md`
       when the change affects users or operators.
    8. Please run `make format` before every commit.
       Additionally, ensure that your code is lint compliant by running `make lint`.
       There are CI tests built into the Moca repository,
       and all PRs require those checks to pass before they can be merged.

**Note**: for very small or obvious problems, such as typos,
it is not always necessary to open an issue before submitting a PR.
For more complex problems or features, a PR opened before design discussion has taken place in GitHub issues
is more likely to be rejected or sent back for clarification.

Looking for a good place to start contributing?
Check out our [good first issues](https://github.com/mocachain/moca/issues?q=label%3A%22good+first+issue%22).

## <span id="adr">Architecture Decision Records (ADR)</span>

When proposing an architecture decision for Moca,
please create an ADR or another written design note
so further discussions can be made before implementation begins.
If you would like to see examples of the general format,
refer to [Tendermint ADRs](https://github.com/tendermint/tendermint/tree/master/docs/architecture).

## <span id="forking">Forking</span>

Go modules make local clone paths flexible,
but it is still useful to keep your remotes organized when working from a fork.

For instance, to create a fork and work on a branch of it, you would:

1. Create the fork on GitHub using the fork button.
2. Go to the original repo checked out locally.
3. `git remote rename origin upstream`
4. `git remote add origin git@github.com:<your-handle>/moca.git`

Now `origin` refers to your fork and `upstream` refers to the main Moca repository.
You can `git push -u origin <branch-name>` to update your fork
and make pull requests to Moca from there.

To pull in updates from the upstream repo, run:

1. `git fetch upstream`
2. `git rebase upstream/main` (or whichever branch you are targeting)

New branches should be rebased before submitting a PR in case there have been changes
to avoid merge commits.

For example, this branch state:

```
          A---B---C new-branch
         /
    D---E---F---G target-branch
            |   |
         (F, G) changes happened after `new-branch` forked
```

should become this after rebase:

```
                  A'--B'--C' new-branch
                 /
    D---E---F---G target-branch
```

More about rebase [here](https://git-scm.com/docs/git-rebase) and
[here](https://www.atlassian.com/git/tutorials/rewriting-history/git-rebase#:~:text=What%20is%20git%20rebase%3F,of%20a%20feature%20branching%20workflow.).

Please do not make pull requests directly from `main`.

## <span id="dependencies">Dependencies</span>

We use [Go Modules](https://github.com/golang/go/wiki/Modules) to manage dependency versions.

Cosmos repositories should stay healthy with normal module resolution,
but if a third-party dependency breaks the build, we can fall back on `go mod tidy -v`
to repair the module graph after dependency updates.

## <span id="protobuf">Protobuf</span>

We use [Protocol Buffers](https://developers.google.com/protocol-buffers) along
with [gogoproto](https://github.com/cosmos/protobuf) to generate code for use in Moca.

For deterministic behavior around Protobuf tooling, everything is containerized using Docker.
Make sure Docker is installed on your machine, or head to [Docker's website](https://docs.docker.com/get-docker/)
to install it.

For formatting code in `.proto` files, you can run `make proto-format`.

For linting and checking breaking changes, we use [buf](https://buf.build/).
You can use `make proto-lint` and `make proto-check-breaking` respectively to lint your proto files
and check for breaking changes.

To generate the protobuf stubs, run `make proto-gen`.
You can also run `make proto-all` to execute all protobuf-related steps sequentially.

In order for imports to properly compile in your IDE,
you may need to manually set your protobuf path in your workspace settings.

For example, in vscode your `.vscode/settings.json` can look like:

```json
{
  "protoc": {
    "options": [
      "--proto_path=${workspaceRoot}/proto",
      "--proto_path=${workspaceRoot}/third_party/proto"
    ]
  }
}
```

## <span id="dev_procedure">Development Procedure</span>

1. The latest state of development is on `main`.
2. `main` must never fail `make lint`, `make test`, `make test-race`, `make test-rpc`, or `make test-import`.
3. Do not force-push to `main`, except when reverting a broken commit under maintainer coordination.
4. Create your feature branch from `main` or from the relevant active `release/v*` branch when appropriate.
5. Before submitting a pull request, rebase on top of the target branch.

## <span id="testing">Testing</span>

Moca uses [GitHub Actions](https://github.com/features/actions) for automated testing.
Run the relevant local test targets before opening or updating a PR whenever possible.

## <span id="updating_doc">Updating Documentation</span>

If you open a PR on the Moca repo, update the relevant documentation in `/docs`
whenever user-facing behavior, operational workflows, or developer setup changes.
Prior to approval, maintainers may request updates to specific docs.

## <span id="braching_model_and_release">Branching Model and Release</span>

User-facing repos should adhere to the [trunk based development branching model](https://trunkbaseddevelopment.com/).

Libraries need not follow the model strictly, but would be wise to.

Moca utilizes [semantic versioning](https://semver.org/).

### <span id="commit_messages">Commit messages</span>

Commit messages should be written in a short, descriptive manner
and be prefixed with tags for the change type and scope, when possible,
according to the [semantic commit](https://gist.github.com/joshbuchea/6f47e86d2510bce28f8e7f42ae84c716) scheme.

For example, a new change to the `bank` module might have the following message:
`feat(bank): add balance query cli command`

### <span id="pr_targeting">PR Targeting</span>

Ensure that you base and target your PR on the correct maintenance branch.

All feature additions should be targeted against `main`.
Bug fixes for an active release line should be targeted against the corresponding `release/v*` branch when applicable.

### <span id="pull_requests">Pull Requests</span>

To accommodate the review process, we suggest that PRs are broken up categorically.
Ideally each PR addresses only a single issue.
As much as possible, code refactoring and cleanup should be submitted as separate PRs from bug fixes
or feature additions.

### <span id="reviewing_prs">Process for reviewing PRs</span>

All PRs require maintainer review before merge.
When reviewing PRs, please use the following review explanations:

1. `LGTM` without an explicit approval means that the changes look good,
   but you have not pulled down the code, run tests locally, or reviewed it thoroughly.
2. `Approval` through the GitHub UI means that you understand the code,
   the documentation or spec is updated in the right places,
   and you have pulled down and tested the code locally.
   In addition:
    * Think through whether any added code could be partially combined with existing code.
    * Think through any potential security or incentive-compatibility issues introduced by the changes.
    * Ensure naming conventions are consistent with the rest of the codebase.
    * Ensure the code lives in a reasonable location, considering dependency structures.
3. If you are only making surface-level reviews, submit notes as `Comments` without adding a review.

### <span id="pull_merge_procedure">Pull Merge Procedure</span>

1. Ensure the pull branch is rebased on the target branch.
2. Run `make test` to ensure that all tests pass.
3. Squash merge the pull request.

### <span id="release_procedure">Release Procedure</span>

1. Start from the branch that will serve as the release source, typically `main` or an active `release/v*` branch.
2. Prepare the release notes and the corresponding `CHANGELOG.md` updates.
3. Run the required validation and release testing before tagging.
4. Tag the release and publish the corresponding GitHub release.
5. Merge or cherry-pick any follow-up fixes according to the active maintenance strategy.
