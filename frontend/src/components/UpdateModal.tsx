import { useAppStore } from '../stores/appStore'

export function UpdateModal() {
  const {
    showUpdateModal,
    updateInfo,
    isCheckingForUpdates,
    dismissUpdateModal,
    downloadAndInstallUpdate,
    openReleasePage,
  } = useAppStore()

  if (!showUpdateModal || !updateInfo) return null

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-zinc-800 rounded-lg shadow-xl max-w-md w-full mx-4 overflow-hidden">
        {/* Header */}
        <div className="px-6 py-4 border-b border-zinc-700">
          <h2 className="text-lg font-semibold text-white flex items-center gap-2">
            <svg className="w-5 h-5 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            Update Available
          </h2>
        </div>

        {/* Content */}
        <div className="px-6 py-4 space-y-4">
          <div className="flex items-center justify-between text-sm">
            <span className="text-zinc-400">Current version:</span>
            <span className="text-zinc-200 font-mono">{updateInfo.currentVersion}</span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-zinc-400">New version:</span>
            <span className="text-green-400 font-mono font-semibold">{updateInfo.latestVersion}</span>
          </div>
          {updateInfo.assetSize > 0 && (
            <div className="flex items-center justify-between text-sm">
              <span className="text-zinc-400">Download size:</span>
              <span className="text-zinc-200">{formatBytes(updateInfo.assetSize)}</span>
            </div>
          )}

          {/* Release Notes */}
          {updateInfo.releaseNotes && (
            <div className="mt-4">
              <h3 className="text-sm font-medium text-zinc-300 mb-2">What's new:</h3>
              <div className="bg-zinc-900 rounded-md p-3 max-h-32 overflow-y-auto">
                <p className="text-xs text-zinc-400 whitespace-pre-wrap">{updateInfo.releaseNotes}</p>
              </div>
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="px-6 py-4 bg-zinc-900 flex gap-3">
          <button
            onClick={dismissUpdateModal}
            className="flex-1 px-4 py-2 text-sm text-zinc-300 hover:text-white hover:bg-zinc-700 rounded-md transition-colors"
          >
            Later
          </button>
          <button
            onClick={() => {
              openReleasePage()
              dismissUpdateModal()
            }}
            className="px-4 py-2 text-sm text-zinc-300 hover:text-white hover:bg-zinc-700 rounded-md transition-colors"
          >
            View Release
          </button>
          <button
            onClick={downloadAndInstallUpdate}
            disabled={isCheckingForUpdates}
            className="flex-1 px-4 py-2 text-sm bg-green-600 hover:bg-green-500 text-white rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed font-medium"
          >
            Update Now
          </button>
        </div>
      </div>
    </div>
  )
}
