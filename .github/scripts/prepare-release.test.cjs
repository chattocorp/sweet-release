const {test} = require('node:test');
const assert = require('node:assert/strict');
const prepare = require('./prepare-release.cjs');

function fixture({object, draft, readError, releaseError, raceObject} = {}) {
  const calls = [];
  const httpError = status => Object.assign(new Error(`HTTP ${status}`), {status});
  const github = {rest: {
    git: {
      async getRef() {
        if (readError) throw httpError(readError);
        if (!object) throw httpError(404);
        return {data: {object}};
      },
      async getTag() { return {data: {object: {type: 'commit', sha: 'release-sha'}}}; },
      async createRef(args) {
        calls.push(args);
        if (raceObject) { object = raceObject; throw httpError(422); }
        object = {type: 'commit', sha: args.sha};
      },
    },
    repos: {
      async getReleaseByTag() {
        if (releaseError) throw httpError(releaseError);
        if (draft === undefined) throw httpError(404);
        return {data: {draft}};
      },
    },
  }};
  return {calls, run: () => prepare({github, owner: 'owner', repo: 'repo', tag: 'v1.2.3', commit: 'release-sha'})};
}

test('create a tag but continue publication when release is absent', async () => {
  const f = fixture();
  assert.equal(await f.run(), false);
  assert.equal(f.calls.length, 1);
  assert.equal(f.calls[0].sha, 'release-sha');
  assert.equal(f.calls[0].ref, 'refs/tags/v1.2.3');
});

test('retry a matching tag with no release or a draft; skip only published', async () => {
  for (const draft of [undefined, true, false]) {
    const f = fixture({object: {type: 'commit', sha: 'release-sha'}, draft});
    assert.equal(await f.run(), draft === false);
    assert.equal(f.calls.length, 0);
  }
});

test('peel annotated tags and reject mismatched or non-commit refs', async () => {
  assert.equal(await fixture({object: {type: 'tag', sha: 'tag-sha'}}).run(), false);
  for (const object of [{type: 'commit', sha: 'different'}, {type: 'blob', sha: 'release-sha'}]) {
    const f = fixture({object});
    await assert.rejects(f.run(), /refusing to move/);
    assert.equal(f.calls.length, 0);
  }
});

test('concurrent creation accepts only the same commit', async () => {
  assert.equal(await fixture({raceObject: {type: 'commit', sha: 'release-sha'}}).run(), false);
  await assert.rejects(fixture({raceObject: {type: 'commit', sha: 'different'}}).run(), /refusing to move/);
});

test('permission and server errors must not be interpreted as absence', async () => {
  for (const status of [403, 500]) {
    const f = fixture({readError: status});
    await assert.rejects(f.run(), new RegExp(`HTTP ${status}`));
    assert.equal(f.calls.length, 0);
    await assert.rejects(fixture({releaseError: status}).run(), new RegExp(`HTTP ${status}`));
  }
});
