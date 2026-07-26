import { useState } from "react"
import { FileTree } from "./components/FileTree"
import { EditorPanel } from "./components/EditorPanel"
import { ThemeToggle } from "./components/ThemeToggle"
import { VersionBadge } from "./components/VersionBadge"
import { PanelRightOpen } from "lucide-react"
import { Button } from "./components/ui/button"

function App() {
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [sidebarOpen, setSidebarOpen] = useState(true)

  return (
    <div className="h-screen flex flex-col bg-white dark:bg-zinc-900 text-zinc-900 dark:text-zinc-100">
      {/* Header */}
      <header className="flex items-center justify-between px-4 py-2 border-b border-zinc-200 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-800/50">
        <div className="flex items-center gap-3">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            title="Dosya ağacını göster/gizle"
          >
            <PanelRightOpen size={18} />
          </Button>
          <h1 className="text-sm font-semibold">Macros Düzenleyici</h1>
        </div>
        <div className="flex items-center gap-2">
          <VersionBadge />
          <ThemeToggle />
        </div>
      </header>

      {/* Main */}
      <div className="flex flex-1 overflow-hidden">
        {/* Sidebar */}
        {sidebarOpen && (
          <aside className="w-64 border-r border-zinc-200 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-800/30">
            <FileTree
              onFileSelect={setSelectedFile}
              selectedFile={selectedFile}
            />
          </aside>
        )}

        {/* Editor */}
        <main className="flex-1 overflow-hidden">
          <EditorPanel filePath={selectedFile} />
        </main>
      </div>
    </div>
  )
}

export default App
