# Release notes

One file per tag, named after it: `v1.13.0.md` for `v1.13.0`.

The release workflow uses the file as the release body when it exists and falls
back to GitHub's generated notes when it does not. Write it before tagging, from
the section for that version in `CHANGELOG.md`, and lead with what changed for
someone installing the release rather than with a list of merged pull requests.

A missing file is not an error. It means the release ships with the generated
notes, which list pull request titles.
