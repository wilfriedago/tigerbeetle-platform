/* eslint-disable */
import * as types from './graphql';



/**
 * Map of all GraphQL operations in the project.
 *
 * This map has several performance disadvantages:
 * 1. It is not tree-shakeable, so it will include all operations in the project.
 * 2. It is not minifiable, so the string of a GraphQL query will be multiple times inside the bundle.
 * 3. It does not support dead code elimination, so it will add unused operations.
 *
 * Therefore it is highly recommended to use the babel or swc plugin for production.
 * Learn more about it here: https://the-guild.dev/graphql/codegen/plugins/presets/preset-client#reducing-bundle-size
 */
type Documents = {
    "\n  fragment AccountSummaryFields on AccountSummary {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n  }\n": typeof types.AccountSummaryFieldsFragmentDoc,
    "\n  fragment AccountFields on Account {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n    openHolds\n  }\n": typeof types.AccountFieldsFragmentDoc,
    "\n  query Accounts($name: String!, $first: Int, $after: String, $prefix: String, $search: String, $asset: String) {\n    ledger(name: $name) {\n      accounts(first: $first, after: $after, prefix: $prefix, search: $search, asset: $asset) {\n        nodes {\n          ...AccountSummaryFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n": typeof types.AccountsDocument,
    "\n  query Account($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n    }\n  }\n": typeof types.AccountDocument,
    "\n  query AccountHistory($name: String!, $address: String!, $asset: String!, $limit: Int) {\n    ledger(name: $name) {\n      accountHistory(address: $address, asset: $asset, limit: $limit) {\n        address\n        asset\n        data {\n          timestamp\n          at\n          balance\n          reserved\n          credits\n          debits\n        }\n      }\n    }\n  }\n": typeof types.AccountHistoryDocument,
    "\n  query Segments($name: String!, $prefix: String) {\n    ledger(name: $name) {\n      segments(prefix: $prefix) {\n        segment\n        prefix\n        accounts\n        assets\n        isAccount\n        hasChildren\n      }\n    }\n  }\n": typeof types.SegmentsDocument,
    "\n  query AggregateBalances($name: String!, $address: String!) {\n    ledger(name: $name) {\n      aggregateBalances(address: $address) {\n        prefix\n        asOf\n        accounts\n        chunks\n        assets {\n          asset\n          accounts\n          balance\n          available\n          reserved\n          incoming\n        }\n      }\n    }\n  }\n": typeof types.AggregateBalancesDocument,
    "\n  query WriteTarget($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n      ruleMatch(address: $address) {\n        rule {\n          ...RuleFields\n        }\n      }\n    }\n  }\n": typeof types.WriteTargetDocument,
    "\n  mutation ReplaceAccountMetadata($input: ReplaceAccountMetadataInput!) {\n    replaceAccountMetadata(input: $input) {\n      ...AccountFields\n    }\n  }\n": typeof types.ReplaceAccountMetadataDocument,
    "\n  fragment HoldFields on Hold {\n    id\n    ledger\n    source\n    destination\n    asset\n    amount\n    capturedAmount\n    status\n    timeoutSeconds\n    createdAt\n    timestamp\n    expiresAt\n    resolvedAt\n    reference\n    eventType {\n      name\n      code\n    }\n    metadata\n    idempotencyKey\n    pendingTransferId\n    resolutionTransferId\n    tbLedger\n  }\n": typeof types.HoldFieldsFragmentDoc,
    "\n  query Holds($name: String!, $first: Int, $after: String, $status: HoldStatus, $address: String) {\n    ledger(name: $name) {\n      holds(first: $first, after: $after, status: $status, address: $address) {\n        nodes {\n          ...HoldFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n      holdCounts {\n        open\n        captured\n        voided\n        expired\n        reserved {\n          asset\n          amount\n        }\n      }\n    }\n  }\n": typeof types.HoldsDocument,
    "\n  query Hold($name: String!, $id: U128!) {\n    ledger(name: $name) {\n      hold(id: $id) {\n        ...HoldFields\n      }\n    }\n  }\n": typeof types.HoldDocument,
    "\n  mutation CreateHold($input: CreateHoldInput!) {\n    createHold(input: $input) {\n      ...HoldFields\n    }\n  }\n": typeof types.CreateHoldDocument,
    "\n  mutation CaptureHold($input: CaptureHoldInput!) {\n    captureHold(input: $input) {\n      ...HoldFields\n    }\n  }\n": typeof types.CaptureHoldDocument,
    "\n  mutation VoidHold($input: VoidHoldInput!) {\n    voidHold(input: $input) {\n      ...HoldFields\n    }\n  }\n": typeof types.VoidHoldDocument,
    "\n  fragment LedgerFields on Ledger {\n    name\n    metadata\n    createdAt\n    assetCodes\n    accountCount\n  }\n": typeof types.LedgerFieldsFragmentDoc,
    "\n  query Ledgers($first: Int, $after: String) {\n    ledgers(first: $first, after: $after) {\n      nodes {\n        ...LedgerFields\n      }\n      pageInfo {\n        endCursor\n      }\n    }\n  }\n": typeof types.LedgersDocument,
    "\n  query Ledger($name: String!) {\n    ledger(name: $name) {\n      ...LedgerFields\n    }\n  }\n": typeof types.LedgerDocument,
    "\n  query LedgerStats($name: String!) {\n    ledger(name: $name) {\n      stats {\n        accounts\n        accountsToday\n        assets {\n          ...AssetSummaryFields\n        }\n        holds {\n          open\n          captured\n          voided\n          expired\n          reserved {\n            asset\n            amount\n          }\n        }\n        declined {\n          total\n          reasons {\n            reason\n            count\n          }\n        }\n      }\n    }\n  }\n": typeof types.LedgerStatsDocument,
    "\n  fragment AssetSummaryFields on AssetSummary {\n    asset\n    tbLedger\n    createdAt\n    accounts\n    issued\n  }\n": typeof types.AssetSummaryFieldsFragmentDoc,
    "\n  query LedgerAssets($name: String!) {\n    ledger(name: $name) {\n      assets {\n        ...AssetSummaryFields\n      }\n    }\n  }\n": typeof types.LedgerAssetsDocument,
    "\n  query Search($name: String!, $q: String!) {\n    ledger(name: $name) {\n      search(q: $q) {\n        query\n        accounts {\n          address\n          assets\n        }\n        transactions {\n          id\n          reference\n          committedAt\n        }\n        holds {\n          id\n          status\n        }\n        requests {\n          key\n          operation\n          status\n        }\n      }\n    }\n  }\n": typeof types.SearchDocument,
    "\n  mutation CreateLedger($input: CreateLedgerInput!) {\n    createLedger(input: $input) {\n      ...LedgerFields\n    }\n  }\n": typeof types.CreateLedgerDocument,
    "\n  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {\n    replaceLedgerMetadata(input: $input) {\n      ...LedgerFields\n    }\n  }\n": typeof types.ReplaceLedgerMetadataDocument,
    "\n  fragment MemberFields on Member {\n    subject\n    email\n    emailVerified\n    name\n    status\n    role\n    addedAt\n    addedBy\n    ledgers {\n      ledger\n      role\n      grantedAt\n      grantedBy\n    }\n    invitation {\n      id\n      sentAt\n      expiresAt\n      expired\n    }\n  }\n": typeof types.MemberFieldsFragmentDoc,
    "\n  query Members {\n    members {\n      ...MemberFields\n    }\n  }\n": typeof types.MembersDocument,
    "\n  mutation InviteMember($input: InviteMemberInput!) {\n    inviteMember(input: $input) {\n      ...MemberFields\n    }\n  }\n": typeof types.InviteMemberDocument,
    "\n  mutation SetMemberRole($input: SetMemberRoleInput!) {\n    setMemberRole(input: $input)\n  }\n": typeof types.SetMemberRoleDocument,
    "\n  mutation RemoveMember($input: MemberInput!) {\n    removeMember(input: $input)\n  }\n": typeof types.RemoveMemberDocument,
    "\n  mutation SetLedgerRole($input: SetLedgerRoleInput!) {\n    setLedgerRole(input: $input)\n  }\n": typeof types.SetLedgerRoleDocument,
    "\n  mutation RevokeLedgerRole($input: RevokeLedgerRoleInput!) {\n    revokeLedgerRole(input: $input)\n  }\n": typeof types.RevokeLedgerRoleDocument,
    "\n  mutation ResendInvitation($input: InvitationInput!) {\n    resendInvitation(input: $input)\n  }\n": typeof types.ResendInvitationDocument,
    "\n  mutation DeleteInvitation($input: InvitationInput!) {\n    deleteInvitation(input: $input)\n  }\n": typeof types.DeleteInvitationDocument,
    "\n  fragment WriteRequestFields on WriteRequest {\n    key\n    operation\n    status\n    createdAt\n    resolvedAt\n    targetId\n    problemType\n    responseStatus\n    requestId\n    bodyHash\n    attempts\n  }\n": typeof types.WriteRequestFieldsFragmentDoc,
    "\n  query WriteRequests($name: String!, $first: Int, $after: String, $status: RequestStatus, $operation: Operation, $search: String) {\n    ledger(name: $name) {\n      requests(first: $first, after: $after, status: $status, operation: $operation, search: $search) {\n        nodes {\n          ...WriteRequestFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n": typeof types.WriteRequestsDocument,
    "\n  query WriteRequest($name: String!, $key: String!) {\n    ledger(name: $name) {\n      request(key: $key) {\n        key\n        operation\n        status\n        createdAt\n        resolvedAt\n        targetId\n        problemType\n        responseStatus\n        requestId\n        bodyHash\n        attempts\n        request\n        response\n        transferIds\n      }\n    }\n  }\n": typeof types.WriteRequestDocument,
    "\n  fragment RuleFields on Rule {\n    pattern\n    limit\n    history\n    code\n    builtIn\n    segments\n    createdAt\n    updatedAt\n  }\n": typeof types.RuleFieldsFragmentDoc,
    "\n  fragment EventTypeFields on EventType {\n    name\n    code\n    builtIn\n    createdAt\n    transactions\n  }\n": typeof types.EventTypeFieldsFragmentDoc,
    "\n  query Rules($name: String!) {\n    ledger(name: $name) {\n      rules {\n        ...RuleFields\n      }\n    }\n  }\n": typeof types.RulesDocument,
    "\n  query RuleMatch($name: String!, $address: String!) {\n    ledger(name: $name) {\n      ruleMatch(address: $address) {\n        address\n        rule {\n          ...RuleFields\n        }\n        candidates {\n          matches\n          rule {\n            ...RuleFields\n          }\n        }\n        existing\n      }\n    }\n  }\n": typeof types.RuleMatchDocument,
    "\n  query EventTypes($name: String!) {\n    ledger(name: $name) {\n      eventTypes {\n        ...EventTypeFields\n      }\n    }\n  }\n": typeof types.EventTypesDocument,
    "\n  mutation PutRule($input: PutRuleInput!) {\n    putRule(input: $input) {\n      ...RuleFields\n    }\n  }\n": typeof types.PutRuleDocument,
    "\n  mutation DeleteRule($input: DeleteRuleInput!) {\n    deleteRule(input: $input)\n  }\n": typeof types.DeleteRuleDocument,
    "\n  mutation CreateEventType($input: CreateEventTypeInput!) {\n    createEventType(input: $input) {\n      ...EventTypeFields\n    }\n  }\n": typeof types.CreateEventTypeDocument,
    "\n  fragment OrganizationFields on Organization {\n    id\n    keycloakId\n    alias\n    name\n    status\n    suspensionReason\n    createdAt\n    createdBy\n    owners {\n      subject\n      email\n      name\n      status\n      since\n    }\n    members\n    ledgers\n  }\n": typeof types.OrganizationFieldsFragmentDoc,
    "\n  query Organizations($first: Int, $after: String) {\n    organizations(first: $first, after: $after) {\n      nodes {\n        ...OrganizationFields\n      }\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n    }\n  }\n": typeof types.OrganizationsDocument,
    "\n  query Organization($alias: String!) {\n    organization(alias: $alias) {\n      ...OrganizationFields\n    }\n  }\n": typeof types.OrganizationDocument,
    "\n  mutation CreateOrganization($input: CreateOrganizationInput!) {\n    createOrganization(input: $input) {\n      ...OrganizationFields\n    }\n  }\n": typeof types.CreateOrganizationDocument,
    "\n  mutation UpdateOrganization($input: UpdateOrganizationInput!) {\n    updateOrganization(input: $input) {\n      ...OrganizationFields\n    }\n  }\n": typeof types.UpdateOrganizationDocument,
    "\n  fragment TransactionFields on Transaction {\n    id\n    ledger\n    postings {\n      source\n      destination\n      asset\n      amount\n      transferId\n      tbLedger\n    }\n    reference\n    eventType {\n      name\n      code\n    }\n    businessTime\n    metadata\n    committedAt\n    timestamp\n    reverts\n    revertedBy\n    idempotencyKey\n    requestId\n  }\n": typeof types.TransactionFieldsFragmentDoc,
    "\n  query Transactions(\n    $name: String!\n    $first: Int\n    $after: String\n    $address: String\n    $asset: String\n    $reference: String\n    $eventType: String\n    $committedAfter: DateTime\n    $committedBefore: DateTime\n  ) {\n    ledger(name: $name) {\n      transactions(\n        first: $first\n        after: $after\n        address: $address\n        asset: $asset\n        reference: $reference\n        eventType: $eventType\n        committedAfter: $committedAfter\n        committedBefore: $committedBefore\n      ) {\n        nodes {\n          ...TransactionFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n": typeof types.TransactionsDocument,
    "\n  query Transaction($name: String!, $id: U128!) {\n    ledger(name: $name) {\n      transaction(id: $id) {\n        ...TransactionFields\n      }\n    }\n  }\n": typeof types.TransactionDocument,
    "\n  mutation CreateTransaction($input: CreateTransactionInput!) {\n    createTransaction(input: $input) {\n      ...TransactionFields\n    }\n  }\n": typeof types.CreateTransactionDocument,
    "\n  mutation RevertTransaction($input: RevertTransactionInput!) {\n    revertTransaction(input: $input) {\n      ...TransactionFields\n    }\n  }\n": typeof types.RevertTransactionDocument,
    "\n  mutation ReplaceTransactionMetadata($input: ReplaceTransactionMetadataInput!) {\n    replaceTransactionMetadata(input: $input) {\n      ...TransactionFields\n    }\n  }\n": typeof types.ReplaceTransactionMetadataDocument,
};
const documents: Documents = {
    "\n  fragment AccountSummaryFields on AccountSummary {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n  }\n": types.AccountSummaryFieldsFragmentDoc,
    "\n  fragment AccountFields on Account {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n    openHolds\n  }\n": types.AccountFieldsFragmentDoc,
    "\n  query Accounts($name: String!, $first: Int, $after: String, $prefix: String, $search: String, $asset: String) {\n    ledger(name: $name) {\n      accounts(first: $first, after: $after, prefix: $prefix, search: $search, asset: $asset) {\n        nodes {\n          ...AccountSummaryFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n": types.AccountsDocument,
    "\n  query Account($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n    }\n  }\n": types.AccountDocument,
    "\n  query AccountHistory($name: String!, $address: String!, $asset: String!, $limit: Int) {\n    ledger(name: $name) {\n      accountHistory(address: $address, asset: $asset, limit: $limit) {\n        address\n        asset\n        data {\n          timestamp\n          at\n          balance\n          reserved\n          credits\n          debits\n        }\n      }\n    }\n  }\n": types.AccountHistoryDocument,
    "\n  query Segments($name: String!, $prefix: String) {\n    ledger(name: $name) {\n      segments(prefix: $prefix) {\n        segment\n        prefix\n        accounts\n        assets\n        isAccount\n        hasChildren\n      }\n    }\n  }\n": types.SegmentsDocument,
    "\n  query AggregateBalances($name: String!, $address: String!) {\n    ledger(name: $name) {\n      aggregateBalances(address: $address) {\n        prefix\n        asOf\n        accounts\n        chunks\n        assets {\n          asset\n          accounts\n          balance\n          available\n          reserved\n          incoming\n        }\n      }\n    }\n  }\n": types.AggregateBalancesDocument,
    "\n  query WriteTarget($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n      ruleMatch(address: $address) {\n        rule {\n          ...RuleFields\n        }\n      }\n    }\n  }\n": types.WriteTargetDocument,
    "\n  mutation ReplaceAccountMetadata($input: ReplaceAccountMetadataInput!) {\n    replaceAccountMetadata(input: $input) {\n      ...AccountFields\n    }\n  }\n": types.ReplaceAccountMetadataDocument,
    "\n  fragment HoldFields on Hold {\n    id\n    ledger\n    source\n    destination\n    asset\n    amount\n    capturedAmount\n    status\n    timeoutSeconds\n    createdAt\n    timestamp\n    expiresAt\n    resolvedAt\n    reference\n    eventType {\n      name\n      code\n    }\n    metadata\n    idempotencyKey\n    pendingTransferId\n    resolutionTransferId\n    tbLedger\n  }\n": types.HoldFieldsFragmentDoc,
    "\n  query Holds($name: String!, $first: Int, $after: String, $status: HoldStatus, $address: String) {\n    ledger(name: $name) {\n      holds(first: $first, after: $after, status: $status, address: $address) {\n        nodes {\n          ...HoldFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n      holdCounts {\n        open\n        captured\n        voided\n        expired\n        reserved {\n          asset\n          amount\n        }\n      }\n    }\n  }\n": types.HoldsDocument,
    "\n  query Hold($name: String!, $id: U128!) {\n    ledger(name: $name) {\n      hold(id: $id) {\n        ...HoldFields\n      }\n    }\n  }\n": types.HoldDocument,
    "\n  mutation CreateHold($input: CreateHoldInput!) {\n    createHold(input: $input) {\n      ...HoldFields\n    }\n  }\n": types.CreateHoldDocument,
    "\n  mutation CaptureHold($input: CaptureHoldInput!) {\n    captureHold(input: $input) {\n      ...HoldFields\n    }\n  }\n": types.CaptureHoldDocument,
    "\n  mutation VoidHold($input: VoidHoldInput!) {\n    voidHold(input: $input) {\n      ...HoldFields\n    }\n  }\n": types.VoidHoldDocument,
    "\n  fragment LedgerFields on Ledger {\n    name\n    metadata\n    createdAt\n    assetCodes\n    accountCount\n  }\n": types.LedgerFieldsFragmentDoc,
    "\n  query Ledgers($first: Int, $after: String) {\n    ledgers(first: $first, after: $after) {\n      nodes {\n        ...LedgerFields\n      }\n      pageInfo {\n        endCursor\n      }\n    }\n  }\n": types.LedgersDocument,
    "\n  query Ledger($name: String!) {\n    ledger(name: $name) {\n      ...LedgerFields\n    }\n  }\n": types.LedgerDocument,
    "\n  query LedgerStats($name: String!) {\n    ledger(name: $name) {\n      stats {\n        accounts\n        accountsToday\n        assets {\n          ...AssetSummaryFields\n        }\n        holds {\n          open\n          captured\n          voided\n          expired\n          reserved {\n            asset\n            amount\n          }\n        }\n        declined {\n          total\n          reasons {\n            reason\n            count\n          }\n        }\n      }\n    }\n  }\n": types.LedgerStatsDocument,
    "\n  fragment AssetSummaryFields on AssetSummary {\n    asset\n    tbLedger\n    createdAt\n    accounts\n    issued\n  }\n": types.AssetSummaryFieldsFragmentDoc,
    "\n  query LedgerAssets($name: String!) {\n    ledger(name: $name) {\n      assets {\n        ...AssetSummaryFields\n      }\n    }\n  }\n": types.LedgerAssetsDocument,
    "\n  query Search($name: String!, $q: String!) {\n    ledger(name: $name) {\n      search(q: $q) {\n        query\n        accounts {\n          address\n          assets\n        }\n        transactions {\n          id\n          reference\n          committedAt\n        }\n        holds {\n          id\n          status\n        }\n        requests {\n          key\n          operation\n          status\n        }\n      }\n    }\n  }\n": types.SearchDocument,
    "\n  mutation CreateLedger($input: CreateLedgerInput!) {\n    createLedger(input: $input) {\n      ...LedgerFields\n    }\n  }\n": types.CreateLedgerDocument,
    "\n  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {\n    replaceLedgerMetadata(input: $input) {\n      ...LedgerFields\n    }\n  }\n": types.ReplaceLedgerMetadataDocument,
    "\n  fragment MemberFields on Member {\n    subject\n    email\n    emailVerified\n    name\n    status\n    role\n    addedAt\n    addedBy\n    ledgers {\n      ledger\n      role\n      grantedAt\n      grantedBy\n    }\n    invitation {\n      id\n      sentAt\n      expiresAt\n      expired\n    }\n  }\n": types.MemberFieldsFragmentDoc,
    "\n  query Members {\n    members {\n      ...MemberFields\n    }\n  }\n": types.MembersDocument,
    "\n  mutation InviteMember($input: InviteMemberInput!) {\n    inviteMember(input: $input) {\n      ...MemberFields\n    }\n  }\n": types.InviteMemberDocument,
    "\n  mutation SetMemberRole($input: SetMemberRoleInput!) {\n    setMemberRole(input: $input)\n  }\n": types.SetMemberRoleDocument,
    "\n  mutation RemoveMember($input: MemberInput!) {\n    removeMember(input: $input)\n  }\n": types.RemoveMemberDocument,
    "\n  mutation SetLedgerRole($input: SetLedgerRoleInput!) {\n    setLedgerRole(input: $input)\n  }\n": types.SetLedgerRoleDocument,
    "\n  mutation RevokeLedgerRole($input: RevokeLedgerRoleInput!) {\n    revokeLedgerRole(input: $input)\n  }\n": types.RevokeLedgerRoleDocument,
    "\n  mutation ResendInvitation($input: InvitationInput!) {\n    resendInvitation(input: $input)\n  }\n": types.ResendInvitationDocument,
    "\n  mutation DeleteInvitation($input: InvitationInput!) {\n    deleteInvitation(input: $input)\n  }\n": types.DeleteInvitationDocument,
    "\n  fragment WriteRequestFields on WriteRequest {\n    key\n    operation\n    status\n    createdAt\n    resolvedAt\n    targetId\n    problemType\n    responseStatus\n    requestId\n    bodyHash\n    attempts\n  }\n": types.WriteRequestFieldsFragmentDoc,
    "\n  query WriteRequests($name: String!, $first: Int, $after: String, $status: RequestStatus, $operation: Operation, $search: String) {\n    ledger(name: $name) {\n      requests(first: $first, after: $after, status: $status, operation: $operation, search: $search) {\n        nodes {\n          ...WriteRequestFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n": types.WriteRequestsDocument,
    "\n  query WriteRequest($name: String!, $key: String!) {\n    ledger(name: $name) {\n      request(key: $key) {\n        key\n        operation\n        status\n        createdAt\n        resolvedAt\n        targetId\n        problemType\n        responseStatus\n        requestId\n        bodyHash\n        attempts\n        request\n        response\n        transferIds\n      }\n    }\n  }\n": types.WriteRequestDocument,
    "\n  fragment RuleFields on Rule {\n    pattern\n    limit\n    history\n    code\n    builtIn\n    segments\n    createdAt\n    updatedAt\n  }\n": types.RuleFieldsFragmentDoc,
    "\n  fragment EventTypeFields on EventType {\n    name\n    code\n    builtIn\n    createdAt\n    transactions\n  }\n": types.EventTypeFieldsFragmentDoc,
    "\n  query Rules($name: String!) {\n    ledger(name: $name) {\n      rules {\n        ...RuleFields\n      }\n    }\n  }\n": types.RulesDocument,
    "\n  query RuleMatch($name: String!, $address: String!) {\n    ledger(name: $name) {\n      ruleMatch(address: $address) {\n        address\n        rule {\n          ...RuleFields\n        }\n        candidates {\n          matches\n          rule {\n            ...RuleFields\n          }\n        }\n        existing\n      }\n    }\n  }\n": types.RuleMatchDocument,
    "\n  query EventTypes($name: String!) {\n    ledger(name: $name) {\n      eventTypes {\n        ...EventTypeFields\n      }\n    }\n  }\n": types.EventTypesDocument,
    "\n  mutation PutRule($input: PutRuleInput!) {\n    putRule(input: $input) {\n      ...RuleFields\n    }\n  }\n": types.PutRuleDocument,
    "\n  mutation DeleteRule($input: DeleteRuleInput!) {\n    deleteRule(input: $input)\n  }\n": types.DeleteRuleDocument,
    "\n  mutation CreateEventType($input: CreateEventTypeInput!) {\n    createEventType(input: $input) {\n      ...EventTypeFields\n    }\n  }\n": types.CreateEventTypeDocument,
    "\n  fragment OrganizationFields on Organization {\n    id\n    keycloakId\n    alias\n    name\n    status\n    suspensionReason\n    createdAt\n    createdBy\n    owners {\n      subject\n      email\n      name\n      status\n      since\n    }\n    members\n    ledgers\n  }\n": types.OrganizationFieldsFragmentDoc,
    "\n  query Organizations($first: Int, $after: String) {\n    organizations(first: $first, after: $after) {\n      nodes {\n        ...OrganizationFields\n      }\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n    }\n  }\n": types.OrganizationsDocument,
    "\n  query Organization($alias: String!) {\n    organization(alias: $alias) {\n      ...OrganizationFields\n    }\n  }\n": types.OrganizationDocument,
    "\n  mutation CreateOrganization($input: CreateOrganizationInput!) {\n    createOrganization(input: $input) {\n      ...OrganizationFields\n    }\n  }\n": types.CreateOrganizationDocument,
    "\n  mutation UpdateOrganization($input: UpdateOrganizationInput!) {\n    updateOrganization(input: $input) {\n      ...OrganizationFields\n    }\n  }\n": types.UpdateOrganizationDocument,
    "\n  fragment TransactionFields on Transaction {\n    id\n    ledger\n    postings {\n      source\n      destination\n      asset\n      amount\n      transferId\n      tbLedger\n    }\n    reference\n    eventType {\n      name\n      code\n    }\n    businessTime\n    metadata\n    committedAt\n    timestamp\n    reverts\n    revertedBy\n    idempotencyKey\n    requestId\n  }\n": types.TransactionFieldsFragmentDoc,
    "\n  query Transactions(\n    $name: String!\n    $first: Int\n    $after: String\n    $address: String\n    $asset: String\n    $reference: String\n    $eventType: String\n    $committedAfter: DateTime\n    $committedBefore: DateTime\n  ) {\n    ledger(name: $name) {\n      transactions(\n        first: $first\n        after: $after\n        address: $address\n        asset: $asset\n        reference: $reference\n        eventType: $eventType\n        committedAfter: $committedAfter\n        committedBefore: $committedBefore\n      ) {\n        nodes {\n          ...TransactionFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n": types.TransactionsDocument,
    "\n  query Transaction($name: String!, $id: U128!) {\n    ledger(name: $name) {\n      transaction(id: $id) {\n        ...TransactionFields\n      }\n    }\n  }\n": types.TransactionDocument,
    "\n  mutation CreateTransaction($input: CreateTransactionInput!) {\n    createTransaction(input: $input) {\n      ...TransactionFields\n    }\n  }\n": types.CreateTransactionDocument,
    "\n  mutation RevertTransaction($input: RevertTransactionInput!) {\n    revertTransaction(input: $input) {\n      ...TransactionFields\n    }\n  }\n": types.RevertTransactionDocument,
    "\n  mutation ReplaceTransactionMetadata($input: ReplaceTransactionMetadataInput!) {\n    replaceTransactionMetadata(input: $input) {\n      ...TransactionFields\n    }\n  }\n": types.ReplaceTransactionMetadataDocument,
};

/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment AccountSummaryFields on AccountSummary {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n  }\n"): typeof import('./graphql').AccountSummaryFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment AccountFields on Account {\n    address\n    createdAt\n    metadata\n    balances {\n      asset\n      accountId\n      tbLedger\n      balance\n      available\n      reserved\n      incoming\n      credits\n      debits\n      limit\n      history\n      code\n      rule\n      createdAt\n    }\n    openHolds\n  }\n"): typeof import('./graphql').AccountFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Accounts($name: String!, $first: Int, $after: String, $prefix: String, $search: String, $asset: String) {\n    ledger(name: $name) {\n      accounts(first: $first, after: $after, prefix: $prefix, search: $search, asset: $asset) {\n        nodes {\n          ...AccountSummaryFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').AccountsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Account($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n    }\n  }\n"): typeof import('./graphql').AccountDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query AccountHistory($name: String!, $address: String!, $asset: String!, $limit: Int) {\n    ledger(name: $name) {\n      accountHistory(address: $address, asset: $asset, limit: $limit) {\n        address\n        asset\n        data {\n          timestamp\n          at\n          balance\n          reserved\n          credits\n          debits\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').AccountHistoryDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Segments($name: String!, $prefix: String) {\n    ledger(name: $name) {\n      segments(prefix: $prefix) {\n        segment\n        prefix\n        accounts\n        assets\n        isAccount\n        hasChildren\n      }\n    }\n  }\n"): typeof import('./graphql').SegmentsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query AggregateBalances($name: String!, $address: String!) {\n    ledger(name: $name) {\n      aggregateBalances(address: $address) {\n        prefix\n        asOf\n        accounts\n        chunks\n        assets {\n          asset\n          accounts\n          balance\n          available\n          reserved\n          incoming\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').AggregateBalancesDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query WriteTarget($name: String!, $address: String!) {\n    ledger(name: $name) {\n      account(address: $address) {\n        ...AccountFields\n      }\n      ruleMatch(address: $address) {\n        rule {\n          ...RuleFields\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').WriteTargetDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation ReplaceAccountMetadata($input: ReplaceAccountMetadataInput!) {\n    replaceAccountMetadata(input: $input) {\n      ...AccountFields\n    }\n  }\n"): typeof import('./graphql').ReplaceAccountMetadataDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment HoldFields on Hold {\n    id\n    ledger\n    source\n    destination\n    asset\n    amount\n    capturedAmount\n    status\n    timeoutSeconds\n    createdAt\n    timestamp\n    expiresAt\n    resolvedAt\n    reference\n    eventType {\n      name\n      code\n    }\n    metadata\n    idempotencyKey\n    pendingTransferId\n    resolutionTransferId\n    tbLedger\n  }\n"): typeof import('./graphql').HoldFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Holds($name: String!, $first: Int, $after: String, $status: HoldStatus, $address: String) {\n    ledger(name: $name) {\n      holds(first: $first, after: $after, status: $status, address: $address) {\n        nodes {\n          ...HoldFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n      holdCounts {\n        open\n        captured\n        voided\n        expired\n        reserved {\n          asset\n          amount\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').HoldsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Hold($name: String!, $id: U128!) {\n    ledger(name: $name) {\n      hold(id: $id) {\n        ...HoldFields\n      }\n    }\n  }\n"): typeof import('./graphql').HoldDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateHold($input: CreateHoldInput!) {\n    createHold(input: $input) {\n      ...HoldFields\n    }\n  }\n"): typeof import('./graphql').CreateHoldDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CaptureHold($input: CaptureHoldInput!) {\n    captureHold(input: $input) {\n      ...HoldFields\n    }\n  }\n"): typeof import('./graphql').CaptureHoldDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation VoidHold($input: VoidHoldInput!) {\n    voidHold(input: $input) {\n      ...HoldFields\n    }\n  }\n"): typeof import('./graphql').VoidHoldDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment LedgerFields on Ledger {\n    name\n    metadata\n    createdAt\n    assetCodes\n    accountCount\n  }\n"): typeof import('./graphql').LedgerFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Ledgers($first: Int, $after: String) {\n    ledgers(first: $first, after: $after) {\n      nodes {\n        ...LedgerFields\n      }\n      pageInfo {\n        endCursor\n      }\n    }\n  }\n"): typeof import('./graphql').LedgersDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Ledger($name: String!) {\n    ledger(name: $name) {\n      ...LedgerFields\n    }\n  }\n"): typeof import('./graphql').LedgerDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query LedgerStats($name: String!) {\n    ledger(name: $name) {\n      stats {\n        accounts\n        accountsToday\n        assets {\n          ...AssetSummaryFields\n        }\n        holds {\n          open\n          captured\n          voided\n          expired\n          reserved {\n            asset\n            amount\n          }\n        }\n        declined {\n          total\n          reasons {\n            reason\n            count\n          }\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').LedgerStatsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment AssetSummaryFields on AssetSummary {\n    asset\n    tbLedger\n    createdAt\n    accounts\n    issued\n  }\n"): typeof import('./graphql').AssetSummaryFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query LedgerAssets($name: String!) {\n    ledger(name: $name) {\n      assets {\n        ...AssetSummaryFields\n      }\n    }\n  }\n"): typeof import('./graphql').LedgerAssetsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Search($name: String!, $q: String!) {\n    ledger(name: $name) {\n      search(q: $q) {\n        query\n        accounts {\n          address\n          assets\n        }\n        transactions {\n          id\n          reference\n          committedAt\n        }\n        holds {\n          id\n          status\n        }\n        requests {\n          key\n          operation\n          status\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').SearchDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateLedger($input: CreateLedgerInput!) {\n    createLedger(input: $input) {\n      ...LedgerFields\n    }\n  }\n"): typeof import('./graphql').CreateLedgerDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation ReplaceLedgerMetadata($input: ReplaceLedgerMetadataInput!) {\n    replaceLedgerMetadata(input: $input) {\n      ...LedgerFields\n    }\n  }\n"): typeof import('./graphql').ReplaceLedgerMetadataDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment MemberFields on Member {\n    subject\n    email\n    emailVerified\n    name\n    status\n    role\n    addedAt\n    addedBy\n    ledgers {\n      ledger\n      role\n      grantedAt\n      grantedBy\n    }\n    invitation {\n      id\n      sentAt\n      expiresAt\n      expired\n    }\n  }\n"): typeof import('./graphql').MemberFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Members {\n    members {\n      ...MemberFields\n    }\n  }\n"): typeof import('./graphql').MembersDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation InviteMember($input: InviteMemberInput!) {\n    inviteMember(input: $input) {\n      ...MemberFields\n    }\n  }\n"): typeof import('./graphql').InviteMemberDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation SetMemberRole($input: SetMemberRoleInput!) {\n    setMemberRole(input: $input)\n  }\n"): typeof import('./graphql').SetMemberRoleDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation RemoveMember($input: MemberInput!) {\n    removeMember(input: $input)\n  }\n"): typeof import('./graphql').RemoveMemberDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation SetLedgerRole($input: SetLedgerRoleInput!) {\n    setLedgerRole(input: $input)\n  }\n"): typeof import('./graphql').SetLedgerRoleDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation RevokeLedgerRole($input: RevokeLedgerRoleInput!) {\n    revokeLedgerRole(input: $input)\n  }\n"): typeof import('./graphql').RevokeLedgerRoleDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation ResendInvitation($input: InvitationInput!) {\n    resendInvitation(input: $input)\n  }\n"): typeof import('./graphql').ResendInvitationDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation DeleteInvitation($input: InvitationInput!) {\n    deleteInvitation(input: $input)\n  }\n"): typeof import('./graphql').DeleteInvitationDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment WriteRequestFields on WriteRequest {\n    key\n    operation\n    status\n    createdAt\n    resolvedAt\n    targetId\n    problemType\n    responseStatus\n    requestId\n    bodyHash\n    attempts\n  }\n"): typeof import('./graphql').WriteRequestFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query WriteRequests($name: String!, $first: Int, $after: String, $status: RequestStatus, $operation: Operation, $search: String) {\n    ledger(name: $name) {\n      requests(first: $first, after: $after, status: $status, operation: $operation, search: $search) {\n        nodes {\n          ...WriteRequestFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').WriteRequestsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query WriteRequest($name: String!, $key: String!) {\n    ledger(name: $name) {\n      request(key: $key) {\n        key\n        operation\n        status\n        createdAt\n        resolvedAt\n        targetId\n        problemType\n        responseStatus\n        requestId\n        bodyHash\n        attempts\n        request\n        response\n        transferIds\n      }\n    }\n  }\n"): typeof import('./graphql').WriteRequestDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment RuleFields on Rule {\n    pattern\n    limit\n    history\n    code\n    builtIn\n    segments\n    createdAt\n    updatedAt\n  }\n"): typeof import('./graphql').RuleFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment EventTypeFields on EventType {\n    name\n    code\n    builtIn\n    createdAt\n    transactions\n  }\n"): typeof import('./graphql').EventTypeFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Rules($name: String!) {\n    ledger(name: $name) {\n      rules {\n        ...RuleFields\n      }\n    }\n  }\n"): typeof import('./graphql').RulesDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query RuleMatch($name: String!, $address: String!) {\n    ledger(name: $name) {\n      ruleMatch(address: $address) {\n        address\n        rule {\n          ...RuleFields\n        }\n        candidates {\n          matches\n          rule {\n            ...RuleFields\n          }\n        }\n        existing\n      }\n    }\n  }\n"): typeof import('./graphql').RuleMatchDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query EventTypes($name: String!) {\n    ledger(name: $name) {\n      eventTypes {\n        ...EventTypeFields\n      }\n    }\n  }\n"): typeof import('./graphql').EventTypesDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation PutRule($input: PutRuleInput!) {\n    putRule(input: $input) {\n      ...RuleFields\n    }\n  }\n"): typeof import('./graphql').PutRuleDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation DeleteRule($input: DeleteRuleInput!) {\n    deleteRule(input: $input)\n  }\n"): typeof import('./graphql').DeleteRuleDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateEventType($input: CreateEventTypeInput!) {\n    createEventType(input: $input) {\n      ...EventTypeFields\n    }\n  }\n"): typeof import('./graphql').CreateEventTypeDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment OrganizationFields on Organization {\n    id\n    keycloakId\n    alias\n    name\n    status\n    suspensionReason\n    createdAt\n    createdBy\n    owners {\n      subject\n      email\n      name\n      status\n      since\n    }\n    members\n    ledgers\n  }\n"): typeof import('./graphql').OrganizationFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Organizations($first: Int, $after: String) {\n    organizations(first: $first, after: $after) {\n      nodes {\n        ...OrganizationFields\n      }\n      pageInfo {\n        hasNextPage\n        endCursor\n      }\n    }\n  }\n"): typeof import('./graphql').OrganizationsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Organization($alias: String!) {\n    organization(alias: $alias) {\n      ...OrganizationFields\n    }\n  }\n"): typeof import('./graphql').OrganizationDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateOrganization($input: CreateOrganizationInput!) {\n    createOrganization(input: $input) {\n      ...OrganizationFields\n    }\n  }\n"): typeof import('./graphql').CreateOrganizationDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation UpdateOrganization($input: UpdateOrganizationInput!) {\n    updateOrganization(input: $input) {\n      ...OrganizationFields\n    }\n  }\n"): typeof import('./graphql').UpdateOrganizationDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  fragment TransactionFields on Transaction {\n    id\n    ledger\n    postings {\n      source\n      destination\n      asset\n      amount\n      transferId\n      tbLedger\n    }\n    reference\n    eventType {\n      name\n      code\n    }\n    businessTime\n    metadata\n    committedAt\n    timestamp\n    reverts\n    revertedBy\n    idempotencyKey\n    requestId\n  }\n"): typeof import('./graphql').TransactionFieldsFragmentDoc;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Transactions(\n    $name: String!\n    $first: Int\n    $after: String\n    $address: String\n    $asset: String\n    $reference: String\n    $eventType: String\n    $committedAfter: DateTime\n    $committedBefore: DateTime\n  ) {\n    ledger(name: $name) {\n      transactions(\n        first: $first\n        after: $after\n        address: $address\n        asset: $asset\n        reference: $reference\n        eventType: $eventType\n        committedAfter: $committedAfter\n        committedBefore: $committedBefore\n      ) {\n        nodes {\n          ...TransactionFields\n        }\n        pageInfo {\n          endCursor\n        }\n      }\n    }\n  }\n"): typeof import('./graphql').TransactionsDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Transaction($name: String!, $id: U128!) {\n    ledger(name: $name) {\n      transaction(id: $id) {\n        ...TransactionFields\n      }\n    }\n  }\n"): typeof import('./graphql').TransactionDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateTransaction($input: CreateTransactionInput!) {\n    createTransaction(input: $input) {\n      ...TransactionFields\n    }\n  }\n"): typeof import('./graphql').CreateTransactionDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation RevertTransaction($input: RevertTransactionInput!) {\n    revertTransaction(input: $input) {\n      ...TransactionFields\n    }\n  }\n"): typeof import('./graphql').RevertTransactionDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation ReplaceTransactionMetadata($input: ReplaceTransactionMetadataInput!) {\n    replaceTransactionMetadata(input: $input) {\n      ...TransactionFields\n    }\n  }\n"): typeof import('./graphql').ReplaceTransactionMetadataDocument;


export function graphql(source: string) {
  return (documents as any)[source] ?? {};
}
