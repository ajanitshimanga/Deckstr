// Hydrate the download button with the latest .exe asset from the GitHub
// releases API. The anchor already defaults to /releases/latest on the
// repo, so users with no JS (or a failed fetch) still land somewhere
// sensible — this just shortcuts the click into a direct download.

(function () {
  const OWNER = 'ajanitshimanga';
  const REPO = 'Deckstr';
  const btn = document.getElementById('download');
  const meta = document.getElementById('download-meta');
  if (!btn || !meta) return;

  fetch(`https://api.github.com/repos/${OWNER}/${REPO}/releases/latest`, {
    headers: { Accept: 'application/vnd.github+json' },
  })
    .then((r) => {
      if (!r.ok) throw new Error(`GitHub API ${r.status}`);
      return r.json();
    })
    .then((release) => {
      const exe = (release.assets || []).find((a) =>
        typeof a.name === 'string' && a.name.toLowerCase().endsWith('.exe'),
      );
      if (!exe || !exe.browser_download_url) return;
      btn.href = exe.browser_download_url;
      btn.setAttribute('download', exe.name);
      if (release.tag_name) {
        meta.textContent = release.tag_name;
      }
    })
    .catch(() => {
      // Leave the fallback href + "latest release" label in place. The
      // default anchor still takes the user to GitHub's latest page.
    });
})();
