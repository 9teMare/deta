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

interface DataOperationsProps {
    account: string;
}

export function DataOperations({ account }: DataOperationsProps) {
    const [datasetId, setDatasetId] = useState("");
    const [loading, setLoading] = useState(false);
    const [activeDatasets, setActiveDatasets] = useState<DatasetInfo[]>([]);
    const [loadingDatasets, setLoadingDatasets] = useState(false);

    const { signAndSubmitTransaction } = useWallet();

    // Load active datasets for deletion dropdown
    const loadActiveDatasets = async () => {
        setLoadingDatasets(true);
        try {
            const vault = await apiClient.getUserVault(account);
            if (vault.datasets.length === 0) {
                setActiveDatasets([]);
                return;
            }

            // Load details for all datasets and filter active ones
            const detailsPromises = vault.datasets.map(async (id) => {
                try {
                    const detail = await apiClient.getDataset(account, id);
                    return detail;
                } catch {
                    return null;
                }
            });

            const details = await Promise.all(detailsPromises);
            const active = details.filter((d) => d !== null && d.is_active) as DatasetInfo[];
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

            console.log("Delete dataset - Account:", accountAddress);
            console.log("Delete dataset - Dataset ID:", datasetIdNum);
            console.log("Delete dataset - Dataset ID type:", typeof datasetIdNum);

            const transaction = await buildTransaction(
                {
                    moduleAddress: "0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab",
                    moduleName: "data_registry",
                    functionName: "delete_dataset",
                    args: [datasetIdNum],
                },
                accountAddress
            );

            console.log("Delete transaction:", JSON.stringify(transaction, null, 2));
            console.log("Transaction function:", transaction?.data?.function);

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
        <div className="space-y-4">
            <CSVUpload account={account} />

            <Card>
                <CardHeader>
                    <div className="flex justify-between items-center">
                        <div>
                            <CardTitle>Delete Dataset</CardTitle>
                            <CardDescription>Delete an active dataset you own</CardDescription>
                        </div>
                        <Button onClick={loadActiveDatasets} disabled={loadingDatasets} variant="outline" size="sm">
                            {loadingDatasets ? "Loading..." : "Refresh"}
                        </Button>
                    </div>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div>
                        <Label htmlFor="datasetId">Dataset ID</Label>
                        {loadingDatasets ? (
                            <p className="text-sm text-muted-foreground py-2">Loading active datasets...</p>
                        ) : activeDatasets.length > 0 ? (
                            <select
                                id="datasetId"
                                className="w-full px-3 py-2 border rounded-md bg-background"
                                value={datasetId}
                                onChange={(e) => setDatasetId(e.target.value)}
                            >
                                <option value="">Select a dataset to delete</option>
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
                                        <option key={dataset.id} value={dataset.id}>
                                            Dataset #{dataset.id} - {description}
                                        </option>
                                    );
                                })}
                            </select>
                        ) : (
                            <>
                                <Input
                                    id="datasetId"
                                    type="number"
                                    placeholder="Enter dataset ID manually"
                                    value={datasetId}
                                    onChange={(e) => setDatasetId(e.target.value)}
                                />
                                <p className="text-xs text-muted-foreground mt-1">No active datasets found. Enter a dataset ID manually.</p>
                            </>
                        )}
                    </div>
                    <Button onClick={handleDeleteDataset} disabled={loading || !datasetId.trim()} variant="destructive" className="w-full">
                        {loading ? "Deleting..." : "Delete Dataset"}
                    </Button>
                    {activeDatasets.length === 0 && !loadingDatasets && (
                        <p className="text-xs text-muted-foreground text-center">
                            All your datasets may already be deleted, or you haven&apos;t created any yet.
                        </p>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
