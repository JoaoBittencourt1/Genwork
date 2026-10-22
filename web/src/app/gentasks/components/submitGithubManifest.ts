import type { GithubManifestStart } from '../domain/types'

// submitGithubManifest leaves the app for GitHub: the manifest flow requires a top-level form POST with a "manifest" field.
export function submitGithubManifest(start: GithubManifestStart): void {
  const form = document.createElement('form')
  form.method = 'post'
  form.action = start.formAction
  const manifest = document.createElement('input')
  manifest.type = 'hidden'
  manifest.name = 'manifest'
  manifest.value = JSON.stringify(start.manifest)
  form.appendChild(manifest)
  document.body.appendChild(form)
  form.submit()
}
