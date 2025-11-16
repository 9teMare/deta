import { Aptos, AptosConfig, Network, AccountAddress, U64, SimpleTransaction, TransactionPayload } from "@aptos-labs/ts-sdk";

// Initialize Aptos client
const config = new AptosConfig({ network: Network.TESTNET });
export const aptosClient = new Aptos(config);

export interface TransactionRequest {
    moduleAddress: string;
    moduleName: string;
    functionName: string;
    args: any[];
}

export async function buildTransaction(params: TransactionRequest, senderAddress: string): Promise<any> {
    const sender = AccountAddress.fromString(senderAddress);

    // Convert args to proper types - use SDK types for building
    // Store original args for later conversion back to plain types
    const originalArgs = [...params.args];
    const typedArgs: any[] = params.args.map((arg) => {
        if (typeof arg === "string" && arg.startsWith("0x") && arg.length > 10) {
            // Likely an address (long hex string)
            return AccountAddress.fromString(arg);
        }
        if (typeof arg === "number") {
            return new U64(BigInt(arg));
        }
        if (typeof arg === "string") {
            const encoder = new TextEncoder();
            return encoder.encode(arg);
        }
        if (arg instanceof Uint8Array) {
            return arg;
        }
        return arg;
    });

    // Build transaction using SDK to get proper structure
    const transaction = await aptosClient.transaction.build.simple({
        sender: sender,
        data: {
            function: `${params.moduleAddress}::${params.moduleName}::${params.functionName}`,
            typeArguments: [],
            functionArguments: typedArgs,
        },
    });

    // The wallet adapter might be checking transaction.data.function
    // Return just the data property which has the function, typeArguments, functionArguments
    // But convert the arguments back to plain types for the wallet adapter
    // Use original args when possible to preserve the original type (especially for numbers)
    const plainArgs = typedArgs.map((arg, index) => {
        // If the original arg was a number, return it as a number
        const originalArg = originalArgs[index];
        if (typeof originalArg === "number") {
            return originalArg;
        }

        if (arg instanceof AccountAddress) {
            return arg.toString();
        }
        if (arg instanceof U64) {
            // For U64, always use the original arg if it was a number
            // This preserves the original type and avoids hex string conversion issues
            if (typeof originalArg === "number") {
                return originalArg;
            }
            // Otherwise, try to parse the U64 hex string (little-endian)
            try {
                const hexString = arg.toString();
                if (hexString.startsWith("0x")) {
                    // Parse little-endian hex: reverse bytes and convert
                    const hexWithoutPrefix = hexString.slice(2);
                    // Little-endian: first byte is least significant
                    let value = BigInt(0);
                    let multiplier = BigInt(1);
                    for (let i = 0; i < hexWithoutPrefix.length; i += 2) {
                        const byte = parseInt(hexWithoutPrefix.slice(i, i + 2), 16);
                        value += BigInt(byte) * multiplier;
                        multiplier = multiplier * BigInt(256);
                    }
                    const num = Number(value);
                    if (!isNaN(num) && num <= Number.MAX_SAFE_INTEGER && num >= 0) {
                        return num;
                    }
                    return value.toString();
                }
                // If it's already a decimal string, convert to number
                const num = Number(hexString);
                if (!isNaN(num) && num <= Number.MAX_SAFE_INTEGER && num >= 0) {
                    return num;
                }
                return hexString;
            } catch (e) {
                // Fallback: use original arg if it was a number
                if (typeof originalArg === "number") {
                    return originalArg;
                }
                return arg.toString();
            }
        }
        if (arg instanceof Uint8Array) {
            return Array.from(arg);
        }
        return arg;
    });

    // Build the function string - ensure all parts are defined
    const moduleAddr = params.moduleAddress || "";
    const moduleName = params.moduleName || "";
    const functionName = params.functionName || "";
    const functionString = `${moduleAddr}::${moduleName}::${functionName}`;

    console.log("buildTransaction - Function string:", functionString);
    console.log("buildTransaction - Module parts:", { moduleAddr, moduleName, functionName });
    console.log("buildTransaction - Args:", plainArgs);
    console.log(
        "buildTransaction - Args types:",
        plainArgs.map((a) => typeof a)
    );

    // The wallet adapter might be checking transaction.data.function
    // Try returning it with a 'data' property wrapper
    // But also try without it - the wallet adapter should handle both
    return {
        data: {
            function: functionString,
            typeArguments: [],
            functionArguments: plainArgs,
        },
    };
}
