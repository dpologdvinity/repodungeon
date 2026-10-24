#!/usr/bin/env bash
# Builds or updates a signed apt repository from .deb files.
#
# usage: build-apt-repo.sh <deb-dir> <repo-dir> <gpg-key-id>
#
#   deb-dir   directory holding the new .deb files
#   repo-dir  repository root (the gh-pages checkout); old packages stay in it
#   key-id    id of the GPG secret key (already imported) used to sign
#
# Layout written to repo-dir, served by GitHub Pages:
#   pool/main/r/repodungeon/*.deb                      the packages
#   dists/stable/main/binary-<arch>/Packages(.gz)      one index per architecture
#   dists/stable/{Release,Release.gpg,InRelease}       signed top-level index
#   repodungeon.gpg                                    public key, for signed-by=
#   index.html, .nojekyll                              install steps; tell Pages not to rewrite files
#
# Needs: dpkg-dev (dpkg-scanpackages), apt-utils (apt-ftparchive), gpg.
set -euo pipefail

deb_dir=${1:?usage: build-apt-repo.sh <deb-dir> <repo-dir> <gpg-key-id>}
repo=${2:?missing repo-dir}
key=${3:?missing gpg-key-id}
archs=(amd64 arm64)

mkdir -p "$repo/pool/main/r/repodungeon"
cp "$deb_dir"/*.deb "$repo/pool/main/r/repodungeon/"

cd "$repo"
for arch in "${archs[@]}"; do
  dir=dists/stable/main/binary-$arch
  mkdir -p "$dir"
  dpkg-scanpackages --arch "$arch" pool > "$dir/Packages" 2>/dev/null
  gzip -9nc "$dir/Packages" > "$dir/Packages.gz"
done

# apt-ftparchive hashes every index file and writes dists/stable/Release.
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin=repodungeon \
  -o APT::FTPArchive::Release::Label=repodungeon \
  -o APT::FTPArchive::Release::Suite=stable \
  -o APT::FTPArchive::Release::Codename=stable \
  -o APT::FTPArchive::Release::Architectures="${archs[*]}" \
  -o APT::FTPArchive::Release::Components=main \
  release dists/stable > dists/stable/Release

gpg --batch --yes --default-key "$key" --armor --detach-sign -o dists/stable/Release.gpg dists/stable/Release
gpg --batch --yes --default-key "$key" --clearsign -o dists/stable/InRelease dists/stable/Release
gpg --batch --yes --export "$key" > repodungeon.gpg

# Pages must serve the files as they are, and visitors get install steps.
touch .nojekyll
cat > index.html <<'HTML'
<!doctype html>
<meta charset="utf-8">
<title>repodungeon apt repository</title>
<h1>repodungeon apt repository</h1>
<pre>
curl -fsSL https://dpologdvinity.github.io/repodungeon/repodungeon.gpg | sudo tee /usr/share/keyrings/repodungeon.gpg &gt;/dev/null
echo "deb [signed-by=/usr/share/keyrings/repodungeon.gpg] https://dpologdvinity.github.io/repodungeon stable main" | sudo tee /etc/apt/sources.list.d/repodungeon.list
sudo apt update &amp;&amp; sudo apt install repodungeon
</pre>
<p><a href="https://github.com/dpologdvinity/repodungeon">Project on GitHub</a></p>
HTML
