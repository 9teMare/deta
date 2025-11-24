import { createClient } from '@supabase/supabase-js'

const supabaseUrl = process.env.NEXT_PUBLIC_SUPABASE_URL || ''
const supabaseAnonKey = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY || ''

if (!supabaseUrl || !supabaseAnonKey) {
    console.warn('Supabase credentials not found. Database features will be disabled.')
}

export const supabase = createClient(supabaseUrl, supabaseAnonKey)

// Access Request Types
export interface AccessRequest {
    id: string
    owner_address: string
    requester_address: string
    dataset_id: number
    status: 'pending' | 'approved' | 'denied' | 'paid'
    message?: string
    price_apt: number
    payment_tx_hash?: string
    created_at: string
    approved_at?: string
    paid_at?: string
}

// Create access request
export async function createAccessRequest(
    ownerAddress: string,
    requesterAddress: string,
    datasetId: number,
    message?: string
): Promise<AccessRequest | null> {
    const { data, error } = await supabase
        .from('access_requests')
        .insert({
            owner_address: ownerAddress,
            requester_address: requesterAddress,
            dataset_id: datasetId,
            status: 'pending',
            message: message || '',
            price_apt: 0.1,
        })
        .select()
        .single()

    if (error) {
        console.error('Error creating access request:', error)
        return null
    }

    return data
}

// Get pending requests for owner
export async function getPendingRequests(ownerAddress: string): Promise<AccessRequest[]> {
    const { data, error } = await supabase
        .from('access_requests')
        .select('*')
        .eq('owner_address', ownerAddress)
        .eq('status', 'pending')
        .order('created_at', { ascending: false })

    if (error) {
        console.error('Error fetching pending requests:', error)
        return []
    }

    return data || []
}

// Get approved requests for requester
export async function getApprovedRequests(requesterAddress: string): Promise<AccessRequest[]> {
    const { data, error } = await supabase
        .from('access_requests')
        .select('*')
        .eq('requester_address', requesterAddress)
        .eq('status', 'approved')
        .order('created_at', { ascending: false })

    if (error) {
        console.error('Error fetching approved requests:', error)
        return []
    }

    return data || []
}

// Approve access request (owner)
export async function approveAccessRequest(
    ownerAddress: string,
    requesterAddress: string,
    datasetId: number
): Promise<boolean> {
    const { error } = await supabase
        .from('access_requests')
        .update({
            status: 'approved',
            approved_at: new Date().toISOString(),
        })
        .eq('owner_address', ownerAddress)
        .eq('requester_address', requesterAddress)
        .eq('dataset_id', datasetId)

    if (error) {
        console.error('Error approving request:', error)
        return false
    }

    return true
}

// Deny access request (owner)
export async function denyAccessRequest(
    ownerAddress: string,
    requesterAddress: string,
    datasetId: number
): Promise<boolean> {
    const { error } = await supabase
        .from('access_requests')
        .update({ status: 'denied' })
        .eq('owner_address', ownerAddress)
        .eq('requester_address', requesterAddress)
        .eq('dataset_id', datasetId)

    if (error) {
        console.error('Error denying request:', error)
        return false
    }

    return true
}

// Confirm payment (requester)
export async function confirmPayment(
    ownerAddress: string,
    requesterAddress: string,
    datasetId: number,
    txHash: string
): Promise<boolean> {
    const { error } = await supabase
        .from('access_requests')
        .update({
            status: 'paid',
            payment_tx_hash: txHash,
            paid_at: new Date().toISOString(),
        })
        .eq('owner_address', ownerAddress)
        .eq('requester_address', requesterAddress)
        .eq('dataset_id', datasetId)
        .eq('status', 'approved')

    if (error) {
        console.error('Error confirming payment:', error)
        return false
    }

    return true
}

// Check if request exists
export async function checkRequestExists(
    ownerAddress: string,
    requesterAddress: string,
    datasetId: number
): Promise<AccessRequest | null> {
    const { data, error } = await supabase
        .from('access_requests')
        .select('*')
        .eq('owner_address', ownerAddress)
        .eq('requester_address', requesterAddress)
        .eq('dataset_id', datasetId)
        .single()

    if (error) {
        return null
    }

    return data
}
