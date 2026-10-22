// A tag identifies the exact source commit. Only a published release indicates
// completion: a tag or a draft may remain after a failed upload.
module.exports = async function prepareRelease({github, owner, repo, tag, commit}) {
  const readTag = async () => {
    let data;
    try {
      ({data} = await github.rest.git.getRef({owner, repo, ref: `tags/${tag}`}));
    } catch (error) {
      if (error.status === 404) return false;
      throw error;
    }
    let object = data.object;
    for (let depth = 0; object.type === 'tag' && depth < 16; depth++) {
      const result = await github.rest.git.getTag({owner, repo, tag_sha: object.sha});
      object = result.data.object;
    }
    if (object.type !== 'commit' || object.sha !== commit) {
      throw new Error(`Tag ${tag} does not point to release commit ${commit}; refusing to move it.`);
    }
    return true;
  };

  if (!await readTag()) {
    try {
      await github.rest.git.createRef({owner, repo, ref: `refs/tags/${tag}`, sha: commit});
    } catch (error) {
      // Accept a concurrent creator only if the tag points to this exact commit.
      if (error.status !== 422 || !await readTag()) throw error;
    }
  }

  try {
    const {data} = await github.rest.repos.getReleaseByTag({owner, repo, tag});
    return !data.draft;
  } catch (error) {
    if (error.status === 404) return false;
    throw error;
  }
};
