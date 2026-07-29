import { useState, useEffect, useCallback } from "react"
import { ChevronRight, ChevronDown, FileIcon, FolderIcon, FolderOpenIcon, Plus, RefreshCw } from "lucide-react"
import Swal from "sweetalert2"
import { ScrollArea } from "./ui/scroll-area"
import { cn } from "../lib/utils"
import { ListDir, ListDirs, CreateFile, MoveFile } from "../../bindings/changeme/macrosservice"
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
  const refreshTree = useCallback(async () => {
    const entries = await ListDir("")
    setRoot({
      name: "Macros",
      path: "",
      isDir: true,
      expanded: true,
      loaded: true,
      children: (entries ?? []).map((e: FileEntry) => ({
        name: e.name,
        path: e.path,
        isDir: e.isDir,
        expanded: false,
        loaded: !e.isDir,
      })),
    })
  }, [])

  const [root, setRoot] = useState<TreeNode>({
    name: "Macros",
    path: "",
    isDir: true,
    expanded: true,
    loaded: false,
  })
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    refreshTree()
  }, [refreshTree])

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
        onContextMenu={!node.isDir ? (e) => { e.preventDefault(); handleMove(node.path) } : undefined}
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

  const handleMove = async (filePath: string) => {
    const isDark = document.documentElement.classList.contains("dark")
    let dirs: string[]
    try {
      dirs = (await ListDirs("")) ?? []
    } catch {
      dirs = []
    }
    dirs.sort()

    let selectedDir: string | null = null
    const bg = isDark ? "#1e1e2e" : "#ffffff"
    const fg = isDark ? "#cdd6f4" : "#1e1e2e"
    const itemBg = isDark ? "#2a2a3c" : "#f4f4f5"
    const selectedBg = isDark ? "#6366f1" : "#6366f1"
    const selectedFg = "#ffffff"

    const dirItems = ["", ...dirs].map((d) => {
      const label = d || "Kok dizin"
      const val = d
      return `<div class="dir-item" data-dir="${val}" style="padding:8px 12px;margin:2px 0;border-radius:6px;cursor:pointer;background:${itemBg};color:${fg}">${label}</div>`
    }).join("")

    const result = await Swal.fire({
      title: "Dosyayı Taşı",
      html: `<div style="max-height:300px;overflow-y:auto">${dirItems}</div>`,
      showCancelButton: true,
      confirmButtonText: "Taşı",
      cancelButtonText: "İptal",
      confirmButtonColor: "#6366f1",
      background: bg,
      color: fg,
      didOpen: () => {
        document.querySelectorAll(".dir-item").forEach((el) => {
          (el as HTMLElement).addEventListener("click", () => {
            document.querySelectorAll(".dir-item").forEach((e) => {
              (e as HTMLElement).style.background = itemBg
              ;(e as HTMLElement).style.color = fg
            })
            ;(el as HTMLElement).style.background = selectedBg
            ;(el as HTMLElement).style.color = selectedFg
            selectedDir = el.getAttribute("data-dir") ?? ""
          })
        })
      },
      preConfirm: () => {
        if (selectedDir === null) {
          Swal.showValidationMessage("Bir klasor secin")
          return false
        }
        return selectedDir
      },
    })
    if (!result.isConfirmed) return
    const targetDir = result.value ?? ""

    try {
      const newPath = await MoveFile(filePath, targetDir)
      await refreshTree()
      onFileSelect(newPath)
    } catch (err: any) {
      Swal.fire({
        icon: "error",
        title: "Hata",
        text: err?.message || "Dosya taşınamadı.",
      })
    }
  }

  const handleCreate = async () => {
    const isDark = document.documentElement.classList.contains("dark")
    const result = await Swal.fire({
      title: "Yeni Macro",
      input: "text",
      inputPlaceholder: "Macro adı (örn: my_macro)",
      showCancelButton: true,
      confirmButtonText: "Oluştur",
      cancelButtonText: "İptal",
      inputValidator: (v) => (v?.trim() ? undefined : "Macro adı gerekli"),
      background: isDark ? "#1e1e2e" : "#ffffff",
      color: isDark ? "#cdd6f4" : "#1e1e2e",
      inputAttributes: {
        style: `background:${isDark ? "#2a2a3c" : "#f4f4f5"};color:${isDark ? "#cdd6f4" : "#18181b"};border:1px solid ${isDark ? "#3f3f5c" : "#e4e4e7"};border-radius:6px;padding:8px 12px`,
      },
      customClass: {
        validationMessage: "swal-validation-dark",
      },
      didOpen: () => {
        if (isDark) {
          document.querySelector(".swal2-validation-message")?.setAttribute("style", `background:#2a2a3c;color:#f38ba8`)
          document.querySelector(".swal2-confirm")?.setAttribute("style", `background:#6366f1;color:#fff`)
        }
      },
    })
    if (!result.isConfirmed) return
    const name = result.value.trim()

    // Seçili dosyanın dizinini parent olarak kullan
    const parent = selectedFile ? selectedFile.substring(0, selectedFile.lastIndexOf("/")) : ""

    try {
      const path = await CreateFile(name, parent)
      await refreshTree()
      onFileSelect(path)
    } catch (err: any) {
      Swal.fire({
        icon: "error",
        title: "Hata",
        text: err?.message || "Macro oluşturulamadı.",
      })
    }
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-3 py-2 border-b border-zinc-200 dark:border-zinc-700">
        <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400 uppercase tracking-wider">
          Macros
        </span>
        <div className="flex items-center gap-1">
          <button
            onClick={handleCreate}
            className="p-1 rounded hover:bg-zinc-200 dark:hover:bg-zinc-700 text-zinc-500 dark:text-zinc-400"
            title="Yeni macro oluştur"
          >
            <Plus size={16} />
          </button>
          {loading && <RefreshCw size={14} className="animate-spin text-zinc-400" />}
        </div>
      </div>
      <ScrollArea className="flex-1 py-1">
        {root.children?.map((child) => renderNode(child, 0))}
      </ScrollArea>
    </div>
  )
}
