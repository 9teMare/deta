const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface ApiResponse<T = any> {
    success: boolean;
    message?: string;
    data?: T;
    error?: string;
}

export interface TransactionResponse {
    hash: string;
    success: boolean;
    message?: string;
}

export interface DatasetInfo {
    id: number;
    owner: string;
    data_hash: string;
    metadata: string;
    created_at: number;
    is_active: boolean;
}

export interface VaultInfo {
    datasets: number[];
    count: number;
}

export interface AccessInfo {
    has_access: boolean;
    expires_at?: number;
}

class ApiClient {
    private baseUrl: string;

    constructor(baseUrl: string = API_BASE_URL) {
        this.baseUrl = baseUrl;
    }

    private async request<T>(endpoint: string, options: RequestInit = {}): Promise<ApiResponse<T>> {
        const url = `${this.baseUrl}${endpoint}`;
        const response = await fetch(url, {
            ...options,
            headers: {
                "Content-Type": "application/json",
                ...options.headers,
            },
        });

        if (!response.ok) {
            const error = await response.json().catch(() => ({ error: "Request failed" }));
            throw new Error(error.error || `HTTP error! status: ${response.status}`);
        }

        return response.json();
    }

    async initializeUser(accountAddress: string): Promise<TransactionResponse> {
        const response = await this.request<TransactionResponse>("/api/v1/users/initialize", {
            method: "POST",
            body: JSON.stringify({ account_address: accountAddress }),
        });
        return response.data!;
    }

    async checkInitialization(userAddress: string): Promise<{ initialized: boolean }> {
        const response = await this.request<{ initialized: boolean }>("/api/v1/users/check-initialization", {
            method: "POST",
            body: JSON.stringify({ user: userAddress }),
        });
        return response.data!;
    }

    async submitCSV(accountAddress: string, csvFile: File, schema: any, dataHash: string): Promise<TransactionResponse> {
        const formData = new FormData();
        formData.append("account_address", accountAddress);
        formData.append("data_hash", dataHash);
        formData.append("schema", JSON.stringify(schema));
        formData.append("csv_file", csvFile); // Send the actual file

        const response = await fetch(`${this.baseUrl}/api/v1/data/submit-csv`, {
            method: "POST",
            body: formData,
        });

        if (!response.ok) {
            const error = await response.json().catch(() => ({ error: "Request failed" }));
            throw new Error(error.error || `HTTP error! status: ${response.status}`);
        }

        const result = await response.json();
        return result.data!;
    }

    async submitData(privateKey: string, dataHash: string, metadata: string): Promise<TransactionResponse> {
        const response = await this.request<TransactionResponse>("/api/v1/data/submit", {
            method: "POST",
            body: JSON.stringify({
                private_key: privateKey,
                data_hash: dataHash,
                metadata: metadata,
            }),
        });
        return response.data!;
    }

    async deleteDataset(privateKey: string, datasetId: number): Promise<TransactionResponse> {
        const response = await this.request<TransactionResponse>("/api/v1/data/delete", {
            method: "POST",
            body: JSON.stringify({
                private_key: privateKey,
                dataset_id: datasetId,
            }),
        });
        return response.data!;
    }

    async getDataset(user: string, datasetId: number): Promise<DatasetInfo> {
        // Ensure datasetId is a valid number
        const numericId = typeof datasetId === "string" ? parseInt(datasetId, 10) : Number(datasetId);
        if (isNaN(numericId) || numericId <= 0) {
            throw new Error(`Invalid dataset ID: ${datasetId}`);
        }

        const requestBody = {
            user: user,
            dataset_id: numericId,
        };

        console.log("getDataset request:", requestBody);

        const response = await this.request<DatasetInfo>("/api/v1/data/get", {
            method: "POST",
            body: JSON.stringify(requestBody),
        });
        return response.data!;
    }

    async grantAccess(privateKey: string, datasetId: number, requester: string, expiresAt: number): Promise<TransactionResponse> {
        const response = await this.request<TransactionResponse>("/api/v1/access/grant", {
            method: "POST",
            body: JSON.stringify({
                private_key: privateKey,
                dataset_id: datasetId,
                requester: requester,
                expires_at: expiresAt,
            }),
        });
        return response.data!;
    }

    async revokeAccess(privateKey: string, datasetId: number, requester: string): Promise<TransactionResponse> {
        const response = await this.request<TransactionResponse>("/api/v1/access/revoke", {
            method: "POST",
            body: JSON.stringify({
                private_key: privateKey,
                dataset_id: datasetId,
                requester: requester,
            }),
        });
        return response.data!;
    }

    async checkAccess(owner: string, datasetId: number, requester: string): Promise<AccessInfo> {
        const response = await this.request<AccessInfo>("/api/v1/access/check", {
            method: "POST",
            body: JSON.stringify({
                owner: owner,
                dataset_id: datasetId,
                requester: requester,
            }),
        });
        return response.data!;
    }

    async getUserVault(user: string): Promise<VaultInfo> {
        const response = await this.request<VaultInfo>("/api/v1/vault/get", {
            method: "POST",
            body: JSON.stringify({
                user: user,
            }),
        });
        return response.data!;
    }

    async getUserDatasetsMetadata(user: string): Promise<Array<{ id: number; metadata: string; is_active: boolean }>> {
        const response = await this.request<Array<{ id: number; metadata: string; is_active: boolean }>>("/api/v1/vault/metadata", {
            method: "POST",
            body: JSON.stringify({
                user: user,
            }),
        });
        return response.data || [];
    }

    async registerToken(privateKey: string): Promise<TransactionResponse> {
        const response = await this.request<TransactionResponse>("/api/v1/token/register", {
            method: "POST",
            body: JSON.stringify({ private_key: privateKey }),
        });
        return response.data!;
    }

    async mintToken(privateKey: string, recipient: string, amount: number): Promise<TransactionResponse> {
        const response = await this.request<TransactionResponse>("/api/v1/token/mint", {
            method: "POST",
            body: JSON.stringify({
                private_key: privateKey,
                recipient: recipient,
                amount: amount,
            }),
        });
        return response.data!;
    }

    async getMarketplaceDatasets(): Promise<any[]> {
        const response = await this.request<any[]>("/api/v1/marketplace/datasets", {
            method: "GET",
        });
        return response.data || [];
    }

    async getAccessRequests(owner: string): Promise<any[]> {
        const response = await this.request<any[]>("/api/v1/marketplace/access-requests", {
            method: "POST",
            body: JSON.stringify({ owner }),
        });
        return response.data || [];
    }

    async requestAccess(owner: string, datasetId: number, requester: string, message?: string): Promise<void> {
        await this.request("/api/v1/marketplace/request-access", {
            method: "POST",
            body: JSON.stringify({
                owner,
                dataset_id: datasetId,
                requester,
                message: message || "",
            }),
        });
    }

    async getCSVData(dataHash: string, owner: string, datasetId: number, requester: string): Promise<string[][]> {
        const response = await this.request<string[][]>("/api/v1/data/get-csv", {
            method: "POST",
            body: JSON.stringify({
                data_hash: dataHash,
                owner,
                dataset_id: datasetId,
                requester,
            }),
        });
        return response.data || [];
    }

    async registerUserForMarketplace(userAddress: string): Promise<void> {
        await this.request("/api/v1/marketplace/register-user", {
            method: "POST",
            body: JSON.stringify({
                user_address: userAddress,
            }),
        });
    }
}

export const apiClient = new ApiClient();
