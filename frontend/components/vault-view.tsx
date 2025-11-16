"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { apiClient, type VaultInfo, type DatasetInfo } from "@/lib/api";

interface VaultViewProps {
    account: string;
}

interface DatasetDetail extends DatasetInfo {
    id: number;
}

export function VaultView({ account }: VaultViewProps) {
    const [vault, setVault] = useState<VaultInfo | null>(null);
    const [datasets, setDatasets] = useState<DatasetDetail[]>([]);
    const [loading, setLoading] = useState(false);
    const [loadingDetails, setLoadingDetails] = useState(false);
    const [viewingCSV, setViewingCSV] = useState<{ datasetId: number; data: string[][] } | null>(null);
    const [loadingCSV, setLoadingCSV] = useState(false);

    const loadVault = async () => {
        setLoading(true);
        try {
            const result = await apiClient.getUserVault(account);
            setVault(result);

            // Load details for each dataset
            if (result.datasets.length > 0) {
                await loadDatasetDetails(result.datasets);
            } else {
                setDatasets([]);
            }
        } catch (error: any) {
            toast.error(error.message || "Failed to load vault");
        } finally {
            setLoading(false);
        }
    };

    const loadDatasetDetails = async (datasetIds: number[]) => {
        setLoadingDetails(true);
        try {
            console.log("Loading details for dataset IDs:", datasetIds);
            const validIds = datasetIds.filter((id) => id != null && !isNaN(Number(id)));
            console.log("Valid dataset IDs:", validIds);

            const detailsPromises = validIds.map(async (id) => {
                try {
                    // Ensure id is a number
                    const numericId = typeof id === "string" ? parseInt(id, 10) : Number(id);
                    if (isNaN(numericId)) {
                        throw new Error(`Invalid dataset ID: ${id}`);
                    }
                    console.log(`Fetching dataset ${numericId}...`);
                    const detail = await apiClient.getDataset(account, numericId);
                    console.log(`Successfully loaded dataset ${numericId}:`, detail);
                    return { ...detail, id: numericId };
                } catch (error) {
                    console.error(`Failed to load dataset ${id}:`, error);
                    // If dataset fetch fails, return minimal info
                    const numericId = typeof id === "string" ? parseInt(id, 10) : Number(id);
                    return {
                        id: numericId,
                        owner: account,
                        data_hash: "",
                        metadata: "",
                        created_at: 0,
                        is_active: false,
                    } as DatasetDetail;
                }
            });

            const details = await Promise.all(detailsPromises);
            console.log("All dataset details loaded:", details);
            // Show all datasets, including inactive ones (they'll be marked as inactive)
            setDatasets(details);
        } catch (error: any) {
            console.error("Failed to load dataset details:", error);
        } finally {
            setLoadingDetails(false);
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

    // Removed formatHash - showing full hash now

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
        if (!metadata || metadata.trim() === "") {
            return "No metadata";
        }

        try {
            if (metadata.startsWith("[")) {
                // BCS bytes - try to decode
                return "Binary metadata (raw bytes)";
            }
            const parsed = JSON.parse(metadata);
            if (parsed.schema) {
                const columns = parsed.schema.map((col: any) => `${col.name} (${col.type})`).join(", ");
                return (
                    <div className="space-y-1">
                        {parsed.description && (
                            <div className="mb-2">
                                <strong>Description:</strong> {parsed.description}
                            </div>
                        )}
                        <div>
                            <strong>Schema:</strong> {parsed.schema.length} columns, {parsed.rowCount || 0} rows
                        </div>
                        <div className="text-xs text-muted-foreground">
                            <strong>Columns:</strong> {columns}
                        </div>
                        {parsed.uploadedAt && (
                            <div className="text-xs text-muted-foreground">
                                <strong>Uploaded:</strong> {new Date(parsed.uploadedAt).toLocaleString()}
                            </div>
                        )}
                    </div>
                );
            }
            return metadata;
        } catch {
            // If it's not JSON, try to display as plain text (might be truncated)
            return metadata.length > 100 ? `${metadata.substring(0, 100)}...` : metadata;
        }
    };

    return (
        <>
            <Card>
                <CardHeader>
                    <CardTitle>My Vault</CardTitle>
                    <CardDescription>View your data vault and datasets</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div className="flex justify-between items-center">
                        <div>
                            <p className="text-sm text-muted-foreground">Total Datasets</p>
                            <p className="text-2xl font-bold">{vault?.count || 0}</p>
                        </div>
                        <Button onClick={loadVault} disabled={loading || loadingDetails} variant="outline">
                            {loading || loadingDetails ? "Loading..." : "Refresh"}
                        </Button>
                    </div>

                    {loadingDetails && <p className="text-sm text-muted-foreground text-center py-4">Loading dataset details...</p>}

                    {!loadingDetails && datasets.length > 0 && (
                        <div className="space-y-4">
                            <Label>Dataset Details</Label>
                            {datasets.map((dataset, index) => {
                                // Check for duplicate hashes
                                const duplicateHash = datasets.some(
                                    (d, i) => i !== index && d.data_hash === dataset.data_hash && d.data_hash !== "0x" && d.data_hash !== ""
                                );

                                return (
                                    <Card key={dataset.id} className="border">
                                        <CardContent className="pt-4">
                                            <div className="space-y-2">
                                                <div className="flex justify-between items-start">
                                                    <div>
                                                        <p className="font-semibold">Dataset ID: {dataset.id}</p>
                                                        <p className="text-xs text-muted-foreground">Created: {formatDate(dataset.created_at)}</p>
                                                    </div>
                                                    <div className="flex flex-col items-end gap-1">
                                                        <span
                                                            className={`px-2 py-1 rounded text-xs ${
                                                                dataset.is_active
                                                                    ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                                                                    : "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
                                                            }`}
                                                        >
                                                            {dataset.is_active ? "Active" : "Inactive"}
                                                        </span>
                                                        {!dataset.is_active && <span className="text-xs text-muted-foreground">(Deleted)</span>}
                                                    </div>
                                                </div>
                                                {duplicateHash && (
                                                    <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded p-2 text-xs text-yellow-800 dark:text-yellow-200">
                                                        ⚠️ Warning: This dataset has the same data hash as another dataset. This usually means the
                                                        same CSV file was uploaded twice.
                                                    </div>
                                                )}
                                                <div className="space-y-1 text-sm">
                                                    <div>
                                                        <span className="font-medium">Data Hash: </span>
                                                        <span className="font-mono text-xs break-all">{dataset.data_hash || "N/A"}</span>
                                                        {dataset.data_hash === "0x" && (
                                                            <span className="text-xs text-red-600 dark:text-red-400 ml-2">
                                                                (Empty hash - possible parsing error)
                                                            </span>
                                                        )}
                                                    </div>
                                                    <div>
                                                        <span className="font-medium">Metadata: </span>
                                                        <div className="text-muted-foreground mt-1">{parseMetadata(dataset.metadata)}</div>
                                                    </div>
                                                </div>
                                                <div className="pt-2">
                                                    <Button size="sm" variant="outline" onClick={() => handleViewCSV(dataset)} disabled={loadingCSV}>
                                                        {loadingCSV ? "Loading..." : "View CSV Data"}
                                                    </Button>
                                                </div>
                                            </div>
                                        </CardContent>
                                    </Card>
                                );
                            })}
                        </div>
                    )}

                    {!loadingDetails && vault && vault.datasets.length === 0 && (
                        <p className="text-sm text-muted-foreground text-center py-4">No datasets in your vault yet</p>
                    )}
                </CardContent>
            </Card>

            {/* CSV Data Viewer Modal */}
            {viewingCSV && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" onClick={() => setViewingCSV(null)}>
                    <Card className="max-w-6xl w-full max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
                        <CardHeader>
                            <div className="flex justify-between items-start">
                                <div>
                                    <CardTitle>CSV Data - Dataset #{viewingCSV.datasetId}</CardTitle>
                                    <CardDescription>{viewingCSV.data.length} rows</CardDescription>
                                </div>
                                <Button variant="ghost" size="sm" onClick={() => setViewingCSV(null)}>
                                    ×
                                </Button>
                            </div>
                        </CardHeader>
                        <CardContent className="flex-1 overflow-auto">
                            <div className="overflow-x-auto">
                                <table className="min-w-full text-sm border rounded">
                                    <thead className="bg-muted sticky top-0">
                                        <tr>
                                            {viewingCSV.data[0]?.map((header, idx) => (
                                                <th key={idx} className="px-3 py-2 text-left border font-medium">
                                                    {header}
                                                </th>
                                            ))}
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {viewingCSV.data.slice(1).map((row, rowIdx) => (
                                            <tr key={rowIdx} className="hover:bg-muted/50">
                                                {row.map((cell, cellIdx) => (
                                                    <td key={cellIdx} className="px-3 py-2 border">
                                                        {cell}
                                                    </td>
                                                ))}
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        </CardContent>
                    </Card>
                </div>
            )}
        </>
    );
}
