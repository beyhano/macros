import { useState, useEffect, useCallback } from "react"
import { ChevronRight, ChevronDown, FileIcon, FolderIcon, FolderOpenIcon, RefreshCw } from "lucide-react"
import { ScrollArea } from "./ui/scroll-area"
import { cn } from "../lib/utils"
import { ListDir } from "../../bindings/changeme/macrosservice"
import type { FileEntry } from "../../bindings/changeme/models"

interface TreeNode {
  name: string
  path: string
  isDir: boolean
  children?: TreeNode[]
  expanded: boolean
  loaded: boolean
}

interface FileTreeProps {
  onFileSelect: (path: string) => void
  selectedFile: string | null
}

export function FileTree({ onFileSelect, selectedFile }: FileTreeProps) {
  const [root, setRoot] = useState<TreeNode>({
    name: "Macros",
    path: "",
    isDir: true,
    expanded: true,
    loaded: false,
  })
  const [loading, setLoading] = useState(false)

  const loadChildren = useCallback(async (node: TreeNode) => {
    if (node.loaded) return
    setLoading(true)
    try {
      const entries = await ListDir(node.path)
      node.children = (entries ?? []).map((e: FileEntry) => ({
        name: e.name,
        path: e.path,
        isDir: e.isDir,
        expanded: false,
        loaded: !e.isDir,
      }))
      node.loaded = true
      setRoot((prev) => ({ ...prev }))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadChildren(root)
  }, [])

  const toggle = (node: TreeNode) => {
    if (!node.isDir) {
      onFileSelect(node.path)
      return
    }
    node.expanded = !node.expanded
    if (node.expanded && !node.loaded) {
      loadChildren(node)
    }
    setRoot((prev) => ({ ...prev }))
  }

  const renderNode = (node: TreeNode, depth: number) => (
    <div key={node.path || "/"}>
      <div
        className={cn(
          "file-tree-item",
          !node.isDir && selectedFile === node.path && "active"
        )}
        style={{ paddingLeft: `${12 + depth * 16}px` }}
        onClick={() => toggle(node)}
      >
        {node.isDir ? (
          <>
            {node.expanded ? (
              <>
                <ChevronDown size={14} className="text-zinc-400 shrink-0" />
                <FolderOpenIcon size={16} className="text-yellow-500 shrink-0" />
              </>
            ) : (
              <>
                <ChevronRight size={14} className="text-zinc-400 shrink-0" />
                <FolderIcon size={16} className="text-yellow-500 shrink-0" />
              </>
            )}
          </>
        ) : (
          <>
            <span className="w-[14px] shrink-0" />
            <FileIcon size={16} className="text-blue-400 shrink-0" />
          </>
        )}
        <span className="truncate">{node.name}</span>
      </div>
      {node.isDir && node.expanded && node.children?.map((child) => renderNode(child, depth + 1))}
    </div>
  )

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-3 py-2 border-b border-zinc-200 dark:border-zinc-700">
        <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400 uppercase tracking-wider">
          Macros
        </span>
        {loading && <RefreshCw size={14} className="animate-spin text-zinc-400" />}
      </div>
      <ScrollArea className="flex-1 py-1">
        {root.children?.map((child) => renderNode(child, 0))}
      </ScrollArea>
    </div>
  )
}
