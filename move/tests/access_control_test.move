#[test_only]
module datax::access_control_test {
    use std::vector;
    use aptos_framework::account;
    use aptos_framework::timestamp;
    use datax::access_control;

    const OWNER: address = @0x1;
    const REQUESTER1: address = @0x2;
    const REQUESTER2: address = @0x3;

    fun setup_owner(): signer {
        account::create_account_for_test(OWNER)
    }

    fun setup_timestamp(aptos_framework: &signer) {
        timestamp::set_time_has_started_for_testing(aptos_framework);
    }

    #[test]
    fun test_init_access_control() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let owner = setup_owner();
        access_control::init(&owner);

        // Verify access list was created
        let access_list = access_control::get_access_list(OWNER, 0);
        assert!(vector::length(&access_list) == 0, 1);
    }

    #[test]
    fun test_grant_access() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let owner = setup_owner();
        access_control::init(&owner);

        let current_time = timestamp::now_seconds();
        let expires_at = current_time + 3600; // 1 hour from now

        access_control::grant_access(&owner, 0, REQUESTER1, expires_at);

        assert!(
            access_control::has_access(OWNER, 0, REQUESTER1) == true,
            2
        );
        assert!(
            access_control::has_access(OWNER, 0, REQUESTER2) == false,
            3
        );
    }

    #[test]
    fun test_revoke_access() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let owner = setup_owner();
        access_control::init(&owner);

        let current_time = timestamp::now_seconds();
        let expires_at = current_time + 3600;

        access_control::grant_access(&owner, 0, REQUESTER1, expires_at);
        assert!(
            access_control::has_access(OWNER, 0, REQUESTER1) == true,
            4
        );

        access_control::revoke_access(&owner, 0, REQUESTER1);
        assert!(
            access_control::has_access(OWNER, 0, REQUESTER1) == false,
            5
        );
    }

    #[test]
    fun test_multiple_requesters() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let owner = setup_owner();
        access_control::init(&owner);

        let current_time = timestamp::now_seconds();
        let expires_at = current_time + 3600;

        access_control::grant_access(&owner, 0, REQUESTER1, expires_at);
        access_control::grant_access(&owner, 0, REQUESTER2, expires_at);
        access_control::grant_access(&owner, 1, REQUESTER1, expires_at);

        assert!(
            access_control::has_access(OWNER, 0, REQUESTER1) == true,
            6
        );
        assert!(
            access_control::has_access(OWNER, 0, REQUESTER2) == true,
            7
        );
        assert!(
            access_control::has_access(OWNER, 1, REQUESTER1) == true,
            8
        );
        assert!(
            access_control::has_access(OWNER, 1, REQUESTER2) == false,
            9
        );
    }

    #[test]
    fun test_update_access() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let owner = setup_owner();
        access_control::init(&owner);

        let current_time = timestamp::now_seconds();
        let expires_at1 = current_time + 3600;
        let expires_at2 = current_time + 7200; // 2 hours

        access_control::grant_access(&owner, 0, REQUESTER1, expires_at1);
        access_control::grant_access(&owner, 0, REQUESTER1, expires_at2); // Update

        // Should still have access with new expiration
        assert!(
            access_control::has_access(OWNER, 0, REQUESTER1) == true,
            10
        );
    }

    #[test]
    fun test_get_access_list() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let owner = setup_owner();
        access_control::init(&owner);

        let current_time = timestamp::now_seconds();
        let expires_at = current_time + 3600;

        access_control::grant_access(&owner, 0, REQUESTER1, expires_at);
        access_control::grant_access(&owner, 0, REQUESTER2, expires_at);
        access_control::grant_access(&owner, 1, REQUESTER1, expires_at);

        let access_list = access_control::get_access_list(OWNER, 0);
        assert!(vector::length(&access_list) == 2, 11);

        let access_list2 = access_control::get_access_list(OWNER, 1);
        assert!(vector::length(&access_list2) == 1, 12);
    }

    #[test]
    fun test_auto_initialization() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let owner = setup_owner();

        // Grant access without explicit init - should auto-initialize
        let current_time = timestamp::now_seconds();
        let expires_at = current_time + 3600;

        access_control::grant_access(&owner, 0, REQUESTER1, expires_at);
        assert!(
            access_control::has_access(OWNER, 0, REQUESTER1) == true,
            13
        );
    }

    #[test]
    fun test_nonexistent_access_list() {
        // Test functions on non-existent access list
        assert!(
            access_control::has_access(OWNER, 0, REQUESTER1) == false,
            14
        );

        let access_list = access_control::get_access_list(OWNER, 0);
        assert!(vector::length(&access_list) == 0, 15);
    }
}

