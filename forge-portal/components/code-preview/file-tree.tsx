"use client";

import { useState } from "react";
import { ChevronRight, ChevronDown, FilePlus, FileEdit } from "lucide-react";
import { cn } from "@/lib/utils";

interface FileEntry {
  path: string;
  action: string;     // "create" | "modify"
  language?: string;
}

interface FileTreeProps {
  files: FileEntry[];
  selectedPath: string;
  onSelect: (path: string) => void;
}

interface TreeNode {
  name: string;
  fullPath: string;
  isDir: boolean;
  children: TreeNode[];
  file?: FileEntry;
}

function buildTree(files: FileEntry[]): TreeNode[] {
  const root: TreeNode[] = [];

  for (const file of files) {
    const parts = file.path.split("/");
    let current = root;
    let pathSoFar = "";

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      pathSoFar = pathSoFar ? `${pathSoFar}/${part}` : part;
      const isLast = i === parts.length - 1;

      let node = current.find((n) => n.name === part);
      if (!node) {
        node = {
          name: part,
          fullPath: pathSoFar,
          isDir: !isLast,
          children: [],
          file: isLast ? file : undefined,
        };
        current.push(node);
      }
      current = node.children;
    }
  }

  return root;
}

function TreeNodeItem({
  node,
  selectedPath,
  onSelect,
  depth = 0,
}: {
  node: TreeNode;
  selectedPath: string;
  onSelect: (path: string) => void;
  depth?: number;
}) {
  const [expanded, setExpanded] = useState(true);

  if (node.isDir) {
    return (
      <div>
        <button
          onClick={() => setExpanded(!expanded)}
          className="flex items-center gap-1 w-full px-2 py-1 text-xs text-muted-foreground hover:text-foreground/70"
          style={{ paddingLeft: `${depth * 12 + 8}px` }}
        >
          {expanded ? (
            <ChevronDown className="h-3 w-3 shrink-0" />
          ) : (
            <ChevronRight className="h-3 w-3 shrink-0" />
          )}
          <span>{node.name}/</span>
        </button>
        {expanded &&
          node.children.map((child) => (
            <TreeNodeItem
              key={child.fullPath}
              node={child}
              selectedPath={selectedPath}
              onSelect={onSelect}
              depth={depth + 1}
            />
          ))}
      </div>
    );
  }

  const isCreate = node.file?.action === "create";
  const isSelected = selectedPath === node.fullPath;

  return (
    <button
      onClick={() => onSelect(node.fullPath)}
      className={cn(
        "flex items-center gap-1.5 w-full px-2 py-1 text-xs transition-colors",
        isSelected
          ? "bg-accent/10 text-foreground"
          : "text-muted-foreground hover:text-foreground/80 hover:bg-muted/50"
      )}
      style={{ paddingLeft: `${depth * 12 + 8}px` }}
    >
      {isCreate ? (
        <FilePlus className="h-3 w-3 text-green-400 shrink-0" />
      ) : (
        <FileEdit className="h-3 w-3 text-yellow-400 shrink-0" />
      )}
      <span className="truncate">{node.name}</span>
    </button>
  );
}

export function FileTree({ files, selectedPath, onSelect }: FileTreeProps) {
  const tree = buildTree(files);

  return (
    <div className="overflow-y-auto">
      {tree.map((node) => (
        <TreeNodeItem
          key={node.fullPath}
          node={node}
          selectedPath={selectedPath}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}
