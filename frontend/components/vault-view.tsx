"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { apiClient, type VaultInfo, type DatasetInfo } from "@/lib/api";
import { Database, FileText, Clock, ChevronDown, ChevronRight, Eye, AlertTriangle, RefreshCw } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";

interface VaultViewProps {
    account: string;
}

interface DatasetDetail extends DatasetInfo {
    id: number;
}

export function VaultView({ account }: VaultViewProps) {
    const [vault, setVault] = useState<VaultInfo | null>(null);
    const [datasets, setDatasets] = useState<Map<number, DatasetDetail>>(new Map());
    const [expandedDatasets, setExpandedDatasets] = useState<Set<number>>(new Set());
    const [loadingDatasets, setLoadingDatasets] = useState<Set<number>>(new Set());
    const [loading, setLoading] = useState(false);
    const [viewingCSV, setViewingCSV] = useState<{ datasetId: number; data: string[][] } | null>(null);
    const [loadingCSV, setLoadingCSV] = useState(false);

    const loadVault = async () => {
        setLoading(true);
        try {
            const result = await apiClient.getUserVault(account);
            setVault(result);
            setDatasets(new Map());
        } catch (error: any) {
            toast.error(error.message || "Failed to load vault");
        } finally {
            setLoading(false);
        }
    };

    const loadDatasetDetail = async (datasetId: number) => {
        if (datasets.has(datasetId) || loadingDatasets.has(datasetId)) return;

        setLoadingDatasets((prev) => new Set(prev).add(datasetId));

        try {
            const numericId = typeof datasetId === "string" ? parseInt(datasetId, 10) : Number(datasetId);
            const detail = await apiClient.getDataset(account, numericId);

            setDatasets((prev) => {
                const newMap = new Map(prev);
                newMap.set(numericId, { ...detail, id: numericId });
                return newMap;
            });
        } catch (error: any) {
            console.error(`Failed to load dataset ${datasetId}:`, error);
            toast.error(`Failed to load dataset ${datasetId}`);
        } finally {
            setLoadingDatasets((prev) => {
                const newSet = new Set(prev);
                newSet.delete(datasetId);
                return newSet;
            });
        }
    };

    const toggleDataset = async (datasetId: number) => {
        const isExpanded = expandedDatasets.has(datasetId);

        if (isExpanded) {
            setExpandedDatasets((prev) => {
                const newSet = new Set(prev);
                newSet.delete(datasetId);
                return newSet;
            });
        } else {
            setExpandedDatasets((prev) => new Set(prev).add(datasetId));
            await loadDatasetDetail(datasetId);
        }
    };

    useEffect(() => {
        if (account) {
            loadVault();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [account]);

    const formatDate = (timestamp: number) => {
        if (!timestamp) return "N/A";
        return new Date(timestamp * 1000).toLocaleString();
    };

    const handleViewCSV = async (dataset: DatasetDetail) => {
        setLoadingCSV(true);
        try {
            const csvData = await apiClient.getCSVData(dataset.data_hash, dataset.owner, dataset.id, account);
            setViewingCSV({ datasetId: dataset.id, data: csvData });
        } catch (error: any) {
            toast.error(error.message || "Failed to load CSV data. You may not have access.");
        } finally {
            setLoadingCSV(false);
        }
    };

    const parseMetadata = (metadata: string) => {
        if (!metadata || metadata.trim() === "") return "No metadata";

        try {
            if (metadata.startsWith("[")) return "Binary metadata (raw bytes)";
            const parsed = JSON.parse(metadata);
            if (parsed.schema) {
                const columns = parsed.schema.map((col: any) => `${col.name} (${col.type})`).join(", ");
                return (
                    <div className="space-y-2 text-sm">
                        {parsed.description && (
                            <div className="p-2 bg-white/5 rounded border border-white/5">
                                <span className="text-gray-400 block text-xs uppercase tracking-wider mb-1">Description</span>
                                <span className="text-gray-200">{parsed.description}</span>
                            </div>
                        )}
                        <div className="grid grid-cols-2 gap-2">
                            <div className="p-2 bg-white/5 rounded border border-white/5">
                                <span className="text-gray-400 block text-xs uppercase tracking-wider mb-1">Stats</span>
                                <span className="text-gray-200">{parsed.schema.length} columns, {parsed.rowCount || 0} rows</span>
                            </div>
                            <div className="p-2 bg-white/5 rounded border border-white/5">
                                <span className="text-gray-400 block text-xs uppercase tracking-wider mb-1">Uploaded</span>
                                <span className="text-gray-200">{parsed.uploadedAt ? new Date(parsed.uploadedAt).toLocaleDateString() : "Unknown"}</span>
                            </div>
                        </div>
                        <div className="p-2 bg-white/5 rounded border border-white/5">
                            <span className="text-gray-400 block text-xs uppercase tracking-wider mb-1">Schema</span>
                            <span className="text-gray-200 font-mono text-xs">{columns}</span>
                        </div>
                    </div>
                );
            }
            return metadata;
        } catch {
            return metadata.length > 100 ? `${metadata.substring(0, 100)}...` : metadata;
        }
    };

    return (
        <>
            <Card className="bg-white/5 backdrop-blur-md border-white/10">
                <CardHeader>
                    <div className="flex justify-between items-center">
                        <div>
                            <CardTitle className="flex items-center gap-2">
                                <Database className="w-5 h-5 text-purple-400" />
                                My Vault
                            </CardTitle>
                            <CardDescription>View and manage your stored datasets</CardDescription>
                        </div>
                        <Button 
                            onClick={loadVault} 
                            disabled={loading} 
                            variant="outline" 
                            size="sm"
                            className="bg-white/5 border-white/10 hover:bg-white/10"
                        >
                            <RefreshCw className={`w-4 h-4 mr-2 ${loading ? "animate-spin" : ""}`} />
                            Refresh
                        </Button>
                    </div>
                </CardHeader>
                <CardContent className="space-y-6">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div className="bg-black/20 p-4 rounded-xl border border-white/5 flex items-center justify-between">
                            <div>
                                <p className="text-sm text-gray-400">Total Datasets</p>
                                <p className="text-3xl font-bold text-white">{vault?.count || 0}</p>
                            </div>
                            <div className="p-3 bg-purple-500/20 rounded-full">
                                <Database className="w-6 h-6 text-purple-400" />
                            </div>
                        </div>
                    </div>

                    {vault && vault.datasets.length > 0 ? (
                        <div className="space-y-3">
                            <Label className="text-gray-300">Your Datasets</Label>
                            {vault.datasets.map((datasetId) => {
                                const dataset = datasets.get(datasetId);
                                const isExpanded = expandedDatasets.has(datasetId);
                                const isLoading = loadingDatasets.has(datasetId);

                                return (
                                    <motion.div 
                                        key={datasetId}
                                        layout
                                        initial={{ opacity: 0, y: 10 }}
                                        animate={{ opacity: 1, y: 0 }}
                                        className="rounded-xl border border-white/5 bg-black/20 overflow-hidden"
                                    >
                                        <div 
                                            className="p-4 flex items-center justify-between cursor-pointer hover:bg-white/5 transition-colors"
                                            onClick={() => toggleDataset(datasetId)}
                                        >
                                            <div className="flex items-center gap-3">
                                                <div className={`p-2 rounded-lg ${isExpanded ? 'bg-blue-500/20 text-blue-400' : 'bg-white/5 text-gray-400'}`}>
                                                    <Database className="w-5 h-5" />
                                                </div>
                                                <div>
                                                    <p className="font-medium text-white">Dataset #{datasetId}</p>
                                                    {dataset && (
                                                        <p className="text-xs text-gray-500 flex items-center gap-1">
                                                            <Clock className="w-3 h-3" />
                                                            {formatDate(dataset.created_at)}
                                                        </p>
                                                    )}
                                                </div>
                                            </div>
                                            <div className="flex items-center gap-3">
                                                {dataset && (
                                                    <span className={`px-2 py-1 rounded text-xs font-medium ${
                                                        dataset.is_active 
                                                            ? "bg-green-500/20 text-green-400 border border-green-500/20" 
                                                            : "bg-red-500/20 text-red-400 border border-red-500/20"
                                                    }`}>
                                                        {dataset.is_active ? "Active" : "Inactive"}
                                                    </span>
                                                )}
                                                {isExpanded ? <ChevronDown className="w-5 h-5 text-gray-400" /> : <ChevronRight className="w-5 h-5 text-gray-400" />}
                                            </div>
                                        </div>

                                        <AnimatePresence>
                                            {isExpanded && (
                                                <motion.div 
                                                    initial={{ height: 0, opacity: 0 }}
                                                    animate={{ height: "auto", opacity: 1 }}
                                                    exit={{ height: 0, opacity: 0 }}
                                                    className="border-t border-white/5"
                                                >
                                                    <div className="p-4 space-y-4">
                                                        {isLoading ? (
                                                            <div className="flex items-center justify-center py-4 text-blue-400 gap-2">
                                                                <RefreshCw className="w-4 h-4 animate-spin" />
                                                                Loading details...
                                                            </div>
                                                        ) : dataset ? (
                                                            <>
                                                                {/* Duplicate Warning */}
                                                                {Array.from(datasets.values()).some(
                                                                    (d) => d.id !== dataset.id && d.data_hash === dataset.data_hash && dataset.data_hash !== "0x"
                                                                ) && (
                                                                    <div className="bg-yellow-500/10 border border-yellow-500/20 rounded-lg p-3 flex items-start gap-3">
                                                                        <AlertTriangle className="w-5 h-5 text-yellow-500 shrink-0" />
                                                                        <p className="text-sm text-yellow-200">
                                                                            Duplicate hash detected. This dataset appears to be identical to another one in your vault.
                                                                        </p>
                                                                    </div>
                                                                )}

                                                                <div className="space-y-4">
                                                                    <div>
                                                                        <Label className="text-xs text-gray-500 uppercase tracking-wider font-semibold mb-1 block">Data Hash</Label>
                                                                        <code className="block bg-black/40 p-2 rounded text-xs font-mono text-blue-300 break-all border border-white/5">
                                                                            {dataset.data_hash || "N/A"}
                                                                        </code>
                                                                    </div>
                                                                    
                                                                    <div>
                                                                        <Label className="text-xs text-gray-500 uppercase tracking-wider font-semibold mb-1 block">Metadata</Label>
                                                                        <div className="text-gray-300 text-sm">
                                                                            {parseMetadata(dataset.metadata)}
                                                                        </div>
                                                                    </div>

                                                                    <div className="pt-2">
                                                                        <Button
                                                                            size="sm"
                                                                            className="w-full bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 border border-blue-500/20"
                                                                            onClick={() => handleViewCSV(dataset)}
                                                                            disabled={loadingCSV}
                                                                        >
                                                                            {loadingCSV ? (
                                                                                <>
                                                                                    <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                                                                    Loading Data...
                                                                                </>
                                                                            ) : (
                                                                                <>
                                                                                    <Eye className="w-4 h-4 mr-2" />
                                                                                    View CSV Data
                                                                                </>
                                                                            )}
                                                                        </Button>
                                                                    </div>
                                                                </div>
                                                            </>
                                                        ) : (
                                                            <div className="text-center py-4 text-red-400 bg-red-500/5 rounded-lg border border-red-500/10">
                                                                Failed to load dataset details
                                                            </div>
                                                        )}
                                                    </div>
                                                </motion.div>
                                            )}
                                        </AnimatePresence>
                                    </motion.div>
                                );
                            })}
                        </div>
                    ) : (
                        <div className="text-center py-12 bg-black/20 rounded-xl border border-white/5 border-dashed">
                            <Database className="w-12 h-12 text-gray-600 mx-auto mb-3" />
                            <p className="text-gray-400">No datasets found in your vault</p>
                            <p className="text-sm text-gray-500 mt-1">Upload a CSV file to get started</p>
                        </div>
                    )}
                </CardContent>
            </Card>

            {/* CSV Data Viewer Modal */}
            <AnimatePresence>
                {viewingCSV && (
                    <motion.div 
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4" 
                        onClick={() => setViewingCSV(null)}
                    >
                        <motion.div 
                            initial={{ scale: 0.9, opacity: 0 }}
                            animate={{ scale: 1, opacity: 1 }}
                            exit={{ scale: 0.9, opacity: 0 }}
                            className="bg-[#0a0a0a] border border-white/10 rounded-xl max-w-6xl w-full max-h-[90vh] overflow-hidden flex flex-col shadow-2xl" 
                            onClick={(e) => e.stopPropagation()}
                        >
                            <div className="p-6 border-b border-white/10 flex justify-between items-start bg-white/5">
                                <div>
                                    <h3 className="text-xl font-bold text-white flex items-center gap-2">
                                        <FileText className="w-5 h-5 text-blue-400" />
                                        Dataset #{viewingCSV.datasetId}
                                    </h3>
                                    <p className="text-sm text-gray-400 mt-1">{viewingCSV.data.length} rows found</p>
                                </div>
                                <Button variant="ghost" size="icon" onClick={() => setViewingCSV(null)} className="text-gray-400 hover:text-white hover:bg-white/10">
                                    <span className="text-2xl">×</span>
                                </Button>
                            </div>
                            <div className="flex-1 overflow-auto p-6 bg-black/40">
                                <div className="overflow-x-auto rounded-lg border border-white/10">
                                    <table className="min-w-full text-sm">
                                        <thead className="bg-white/5 sticky top-0">
                                            <tr>
                                                {viewingCSV.data[0]?.map((header, idx) => (
                                                    <th key={idx} className="px-4 py-3 text-left font-medium text-gray-300 border-b border-white/10 whitespace-nowrap">
                                                        {header}
                                                    </th>
                                                ))}
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-white/5">
                                            {viewingCSV.data.slice(1).map((row, rowIdx) => (
                                                <tr key={rowIdx} className="hover:bg-white/5 transition-colors">
                                                    {row.map((cell, cellIdx) => (
                                                        <td key={cellIdx} className="px-4 py-3 text-gray-400 border-b border-white/5 whitespace-nowrap max-w-[300px] truncate">
                                                            {cell}
                                                        </td>
                                                    ))}
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </div>
                        </motion.div>
                    </motion.div>
                )}
            </AnimatePresence>
        </>
    );
}
