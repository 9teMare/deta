"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { useWallet } from "@aptos-labs/wallet-adapter-react";
import { buildTransaction } from "@/lib/aptos-client";
import { CSVUpload } from "@/components/csv-upload";
import { apiClient, type DatasetInfo } from "@/lib/api";
import { Trash2, RefreshCw, Database } from "lucide-react";
import { DATAX_MODULE_ADDRESS } from "@/constants";

interface DataOperationsProps {
    account: string;
}

export function DataOperations({ account }: DataOperationsProps) {
    const [datasetId, setDatasetId] = useState("");
    const [loading, setLoading] = useState(false);
    const [activeDatasets, setActiveDatasets] = useState<DatasetInfo[]>([]);
    const [loadingDatasets, setLoadingDatasets] = useState(false);

    const { signAndSubmitTransaction } = useWallet();

    // Load active datasets for deletion dropdown (using batch endpoint)
    const loadActiveDatasets = async () => {
        setLoadingDatasets(true);
        try {
            // Use the new batch endpoint to get all dataset metadata in one request
            const metadata = await apiClient.getUserDatasetsMetadata(account);

            // Filter active datasets and convert to DatasetInfo format
            const active = metadata
                .filter((d) => d.is_active)
                .map((d) => ({
                    id: d.id,
                    owner: account,
                    data_hash: "",
                    metadata: d.metadata,
                    created_at: 0,
                    is_active: d.is_active,
                })) as DatasetInfo[];

            setActiveDatasets(active);
        } catch (error) {
            console.error("Failed to load active datasets:", error);
            setActiveDatasets([]);
        } finally {
            setLoadingDatasets(false);
        }
    };

    useEffect(() => {
        if (account) {
            loadActiveDatasets();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [account]);

    const handleDeleteDataset = async () => {
        if (!datasetId.trim()) {
            toast.error("Please enter a dataset ID");
            return;
        }

        const datasetIdNum = parseInt(datasetId);
        if (isNaN(datasetIdNum) || datasetIdNum < 0) {
            toast.error("Please enter a valid dataset ID");
            return;
        }

        if (!signAndSubmitTransaction) {
            toast.error("Wallet does not support transaction signing");
            return;
        }

        setLoading(true);
        try {
            // First, verify the dataset exists and is owned by the user
            try {
                const dataset = await apiClient.getDataset(account, datasetIdNum);
                if (!dataset.is_active) {
                    toast.error("Dataset is already deleted (inactive)");
                    setLoading(false);
                    return;
                }
            } catch (err) {
                // If we can't fetch the dataset, it might not exist
                // But we'll still try to delete it (the blockchain will reject if invalid)
                console.warn("Could not verify dataset before deletion:", err);
            }

            // Ensure account is a string
            const accountAddress = typeof account === "string" ? account : (account as any)?.address?.toString() || String(account);

            const transaction = await buildTransaction(
                {
                    moduleAddress: DATAX_MODULE_ADDRESS,
                    moduleName: "data_registry",
                    functionName: "delete_dataset",
                    args: [datasetIdNum],
                },
                accountAddress
            );

            const response = await signAndSubmitTransaction(transaction);

            toast.success(`Dataset deleted! Transaction: ${response.hash}`);
            setDatasetId("");
            // Reload active datasets after successful deletion
            await loadActiveDatasets();
        } catch (error: any) {
            const errorMsg = error.message || "Failed to delete dataset";
            if (errorMsg.includes("abort") || errorMsg.includes("0x3")) {
                toast.error("Dataset not found, already deleted, or you don't own it. Error: " + errorMsg);
            } else {
                toast.error(errorMsg);
            }
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="space-y-6">
            <CSVUpload account={account} />

            <Card className="bg-white/5 backdrop-blur-md border-white/10">
                <CardHeader>
                    <div className="flex justify-between items-center">
                        <div>
                            <CardTitle className="flex items-center gap-2">
                                <Trash2 className="w-5 h-5 text-red-400" />
                                Delete Dataset
                            </CardTitle>
                            <CardDescription>Permanently remove a dataset from the registry</CardDescription>
                        </div>
                        <Button
                            onClick={loadActiveDatasets}
                            disabled={loadingDatasets}
                            variant="outline"
                            size="sm"
                            className="bg-white/5 border-white/10 hover:bg-white/10"
                        >
                            <RefreshCw className={`w-4 h-4 mr-2 ${loadingDatasets ? "animate-spin" : ""}`} />
                            Refresh
                        </Button>
                    </div>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div className="bg-black/20 p-4 rounded-lg border border-white/5">
                        <Label htmlFor="datasetId" className="text-gray-300 mb-2 block">
                            Select Dataset
                        </Label>
                        {loadingDatasets ? (
                            <p className="text-sm text-muted-foreground py-2 flex items-center gap-2">
                                <RefreshCw className="w-3 h-3 animate-spin" /> Loading active datasets...
                            </p>
                        ) : activeDatasets.length > 0 ? (
                            <div className="relative">
                                <select
                                    id="datasetId"
                                    className="w-full px-3 py-2 border border-white/10 rounded-md bg-black/40 text-white appearance-none focus:ring-2 focus:ring-red-500/50 focus:border-red-500/50 transition-all"
                                    value={datasetId}
                                    onChange={(e) => setDatasetId(e.target.value)}
                                >
                                    <option value="" className="bg-gray-900">
                                        Select a dataset to delete
                                    </option>
                                    {activeDatasets.map((dataset) => {
                                        let description = "No description";
                                        try {
                                            if (dataset.metadata) {
                                                const meta = JSON.parse(dataset.metadata);
                                                description = meta.description || "No description";
                                            }
                                        } catch {
                                            description = "No description";
                                        }
                                        return (
                                            <option key={dataset.id} value={dataset.id} className="bg-gray-900">
                                                Dataset #{dataset.id} - {description.substring(0, 30)}
                                                {description.length > 30 ? "..." : ""}
                                            </option>
                                        );
                                    })}
                                </select>
                                <Database className="absolute right-3 top-2.5 w-4 h-4 text-gray-500 pointer-events-none" />
                            </div>
                        ) : (
                            <>
                                <Input
                                    id="datasetId"
                                    type="number"
                                    placeholder="Enter dataset ID manually"
                                    value={datasetId}
                                    onChange={(e) => setDatasetId(e.target.value)}
                                    className="bg-black/40 border-white/10 focus:border-red-500/50 focus:ring-red-500/50"
                                />
                                <p className="text-xs text-muted-foreground mt-2">No active datasets found. Enter a dataset ID manually.</p>
                            </>
                        )}
                    </div>

                    <Button
                        onClick={handleDeleteDataset}
                        disabled={loading || !datasetId.trim()}
                        variant="destructive"
                        className="w-full bg-red-500/20 hover:bg-red-500/30 text-red-400 border border-red-500/20"
                    >
                        {loading ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                Deleting...
                            </>
                        ) : (
                            <>
                                <Trash2 className="w-4 h-4 mr-2" />
                                Delete Dataset
                            </>
                        )}
                    </Button>

                    {activeDatasets.length === 0 && !loadingDatasets && (
                        <p className="text-xs text-gray-500 text-center">
                            All your datasets may already be deleted, or you haven&apos;t created any yet.
                        </p>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
