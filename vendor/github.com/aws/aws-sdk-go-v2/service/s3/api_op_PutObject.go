// Code generated by smithy-go-codegen DO NOT EDIT.

package s3

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	internalChecksum "github.com/aws/aws-sdk-go-v2/service/internal/checksum"
	s3cust "github.com/aws/aws-sdk-go-v2/service/s3/internal/customizations"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"io"
	"time"
)

// Adds an object to a bucket.
//   - Amazon S3 never adds partial objects; if you receive a success response,
//     Amazon S3 added the entire object to the bucket. You cannot use PutObject to
//     only update a single piece of metadata for an existing object. You must put the
//     entire object with updated metadata if you want to update some values.
//   - If your bucket uses the bucket owner enforced setting for Object Ownership,
//     ACLs are disabled and no longer affect permissions. All objects written to the
//     bucket by any account will be owned by the bucket owner.
//   - Directory buckets - For directory buckets, you must make requests for this
//     API operation to the Zonal endpoint. These endpoints support
//     virtual-hosted-style requests in the format
//     https://bucket_name.s3express-az_id.region.amazonaws.com/key-name .
//     Path-style requests are not supported. For more information, see Regional and
//     Zonal endpoints (https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-express-Regions-and-Zones.html)
//     in the Amazon S3 User Guide.
//
// Amazon S3 is a distributed system. If it receives multiple write requests for
// the same object simultaneously, it overwrites all but the last object written.
// However, Amazon S3 provides features that can modify this behavior:
//   - S3 Object Lock - To prevent objects from being deleted or overwritten, you
//     can use Amazon S3 Object Lock (https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html)
//     in the Amazon S3 User Guide. This functionality is not supported for directory
//     buckets.
//   - S3 Versioning - When you enable versioning for a bucket, if Amazon S3
//     receives multiple write requests for the same object simultaneously, it stores
//     all versions of the objects. For each write request that is made to the same
//     object, Amazon S3 automatically generates a unique version ID of that object
//     being stored in Amazon S3. You can retrieve, replace, or delete any version of
//     the object. For more information about versioning, see Adding Objects to
//     Versioning-Enabled Buckets (https://docs.aws.amazon.com/AmazonS3/latest/dev/AddingObjectstoVersioningEnabledBuckets.html)
//     in the Amazon S3 User Guide. For information about returning the versioning
//     state of a bucket, see GetBucketVersioning (https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetBucketVersioning.html)
//     . This functionality is not supported for directory buckets.
//
// Permissions
//   - General purpose bucket permissions - The following permissions are required
//     in your policies when your PutObject request includes specific headers.
//   - s3:PutObject - To successfully complete the PutObject request, you must
//     always have the s3:PutObject permission on a bucket to add an object to it.
//   - s3:PutObjectAcl - To successfully change the objects ACL of your PutObject
//     request, you must have the s3:PutObjectAcl .
//   - s3:PutObjectTagging - To successfully set the tag-set with your PutObject
//     request, you must have the s3:PutObjectTagging .
//   - Directory bucket permissions - To grant access to this API operation on a
//     directory bucket, we recommend that you use the CreateSession (https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateSession.html)
//     API operation for session-based authorization. Specifically, you grant the
//     s3express:CreateSession permission to the directory bucket in a bucket policy
//     or an IAM identity-based policy. Then, you make the CreateSession API call on
//     the bucket to obtain a session token. With the session token in your request
//     header, you can make API requests to this operation. After the session token
//     expires, you make another CreateSession API call to generate a new session
//     token for use. Amazon Web Services CLI or SDKs create session and refresh the
//     session token automatically to avoid service interruptions when a session
//     expires. For more information about authorization, see CreateSession (https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateSession.html)
//     .
//
// Data integrity with Content-MD5
//   - General purpose bucket - To ensure that data is not corrupted traversing
//     the network, use the Content-MD5 header. When you use this header, Amazon S3
//     checks the object against the provided MD5 value and, if they do not match,
//     Amazon S3 returns an error. Alternatively, when the object's ETag is its MD5
//     digest, you can calculate the MD5 while putting the object to Amazon S3 and
//     compare the returned ETag to the calculated MD5 value.
//   - Directory bucket - This functionality is not supported for directory
//     buckets.
//
// HTTP Host header syntax Directory buckets - The HTTP Host header syntax is
// Bucket_name.s3express-az_id.region.amazonaws.com . For more information about
// related Amazon S3 APIs, see the following:
//   - CopyObject (https://docs.aws.amazon.com/AmazonS3/latest/API/API_CopyObject.html)
//   - DeleteObject (https://docs.aws.amazon.com/AmazonS3/latest/API/API_DeleteObject.html)
func (c *Client) PutObject(ctx context.Context, params *PutObjectInput, optFns ...func(*Options)) (*PutObjectOutput, error) {
	if params == nil {
		params = &PutObjectInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "PutObject", params, optFns, c.addOperationPutObjectMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*PutObjectOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type PutObjectInput struct {

	// The bucket name to which the PUT action was initiated. Directory buckets - When
	// you use this operation with a directory bucket, you must use
	// virtual-hosted-style requests in the format
	// Bucket_name.s3express-az_id.region.amazonaws.com . Path-style requests are not
	// supported. Directory bucket names must be unique in the chosen Availability
	// Zone. Bucket names must follow the format bucket_base_name--az-id--x-s3 (for
	// example, DOC-EXAMPLE-BUCKET--usw2-az1--x-s3 ). For information about bucket
	// naming restrictions, see Directory bucket naming rules (https://docs.aws.amazon.com/AmazonS3/latest/userguide/directory-bucket-naming-rules.html)
	// in the Amazon S3 User Guide. Access points - When you use this action with an
	// access point, you must provide the alias of the access point in place of the
	// bucket name or specify the access point ARN. When using the access point ARN,
	// you must direct requests to the access point hostname. The access point hostname
	// takes the form AccessPointName-AccountId.s3-accesspoint.Region.amazonaws.com.
	// When using this action with an access point through the Amazon Web Services
	// SDKs, you provide the access point ARN in place of the bucket name. For more
	// information about access point ARNs, see Using access points (https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-access-points.html)
	// in the Amazon S3 User Guide. Access points and Object Lambda access points are
	// not supported by directory buckets. S3 on Outposts - When you use this action
	// with Amazon S3 on Outposts, you must direct requests to the S3 on Outposts
	// hostname. The S3 on Outposts hostname takes the form
	// AccessPointName-AccountId.outpostID.s3-outposts.Region.amazonaws.com . When you
	// use this action with S3 on Outposts through the Amazon Web Services SDKs, you
	// provide the Outposts access point ARN in place of the bucket name. For more
	// information about S3 on Outposts ARNs, see What is S3 on Outposts? (https://docs.aws.amazon.com/AmazonS3/latest/userguide/S3onOutposts.html)
	// in the Amazon S3 User Guide.
	//
	// This member is required.
	Bucket *string

	// Object key for which the PUT action was initiated.
	//
	// This member is required.
	Key *string

	// The canned ACL to apply to the object. For more information, see Canned ACL (https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#CannedACL)
	// in the Amazon S3 User Guide. When adding a new object, you can use headers to
	// grant ACL-based permissions to individual Amazon Web Services accounts or to
	// predefined groups defined by Amazon S3. These permissions are then added to the
	// ACL on the object. By default, all objects are private. Only the owner has full
	// access control. For more information, see Access Control List (ACL) Overview (https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html)
	// and Managing ACLs Using the REST API (https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-using-rest-api.html)
	// in the Amazon S3 User Guide. If the bucket that you're uploading objects to uses
	// the bucket owner enforced setting for S3 Object Ownership, ACLs are disabled and
	// no longer affect permissions. Buckets that use this setting only accept PUT
	// requests that don't specify an ACL or PUT requests that specify bucket owner
	// full control ACLs, such as the bucket-owner-full-control canned ACL or an
	// equivalent form of this ACL expressed in the XML format. PUT requests that
	// contain other ACLs (for example, custom grants to certain Amazon Web Services
	// accounts) fail and return a 400 error with the error code
	// AccessControlListNotSupported . For more information, see  Controlling
	// ownership of objects and disabling ACLs (https://docs.aws.amazon.com/AmazonS3/latest/userguide/about-object-ownership.html)
	// in the Amazon S3 User Guide.
	//   - This functionality is not supported for directory buckets.
	//   - This functionality is not supported for Amazon S3 on Outposts.
	ACL types.ObjectCannedACL

	// Object data.
	Body io.Reader

	// Specifies whether Amazon S3 should use an S3 Bucket Key for object encryption
	// with server-side encryption using Key Management Service (KMS) keys (SSE-KMS).
	// Setting this header to true causes Amazon S3 to use an S3 Bucket Key for object
	// encryption with SSE-KMS. Specifying this header with a PUT action doesn’t affect
	// bucket-level settings for S3 Bucket Key. This functionality is not supported for
	// directory buckets.
	BucketKeyEnabled *bool

	// Can be used to specify caching behavior along the request/reply chain. For more
	// information, see http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9 (http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9)
	// .
	CacheControl *string

	// Indicates the algorithm used to create the checksum for the object when you use
	// the SDK. This header will not provide any additional functionality if you don't
	// use the SDK. When you send this header, there must be a corresponding
	// x-amz-checksum-algorithm or x-amz-trailer header sent. Otherwise, Amazon S3
	// fails the request with the HTTP status code 400 Bad Request . For the
	// x-amz-checksum-algorithm header, replace  algorithm  with the supported
	// algorithm from the following list:
	//   - CRC32
	//   - CRC32C
	//   - SHA1
	//   - SHA256
	// For more information, see Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html)
	// in the Amazon S3 User Guide. If the individual checksum value you provide
	// through x-amz-checksum-algorithm  doesn't match the checksum algorithm you set
	// through x-amz-sdk-checksum-algorithm , Amazon S3 ignores any provided
	// ChecksumAlgorithm parameter and uses the checksum algorithm that matches the
	// provided value in x-amz-checksum-algorithm . For directory buckets, when you
	// use Amazon Web Services SDKs, CRC32 is the default checksum algorithm that's
	// used for performance.
	ChecksumAlgorithm types.ChecksumAlgorithm

	// This header can be used as a data integrity check to verify that the data
	// received is the same data that was originally sent. This header specifies the
	// base64-encoded, 32-bit CRC32 checksum of the object. For more information, see
	// Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html)
	// in the Amazon S3 User Guide.
	ChecksumCRC32 *string

	// This header can be used as a data integrity check to verify that the data
	// received is the same data that was originally sent. This header specifies the
	// base64-encoded, 32-bit CRC32C checksum of the object. For more information, see
	// Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html)
	// in the Amazon S3 User Guide.
	ChecksumCRC32C *string

	// This header can be used as a data integrity check to verify that the data
	// received is the same data that was originally sent. This header specifies the
	// base64-encoded, 160-bit SHA-1 digest of the object. For more information, see
	// Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html)
	// in the Amazon S3 User Guide.
	ChecksumSHA1 *string

	// This header can be used as a data integrity check to verify that the data
	// received is the same data that was originally sent. This header specifies the
	// base64-encoded, 256-bit SHA-256 digest of the object. For more information, see
	// Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html)
	// in the Amazon S3 User Guide.
	ChecksumSHA256 *string

	// Specifies presentational information for the object. For more information, see
	// https://www.rfc-editor.org/rfc/rfc6266#section-4 (https://www.rfc-editor.org/rfc/rfc6266#section-4)
	// .
	ContentDisposition *string

	// Specifies what content encodings have been applied to the object and thus what
	// decoding mechanisms must be applied to obtain the media-type referenced by the
	// Content-Type header field. For more information, see
	// https://www.rfc-editor.org/rfc/rfc9110.html#field.content-encoding (https://www.rfc-editor.org/rfc/rfc9110.html#field.content-encoding)
	// .
	ContentEncoding *string

	// The language the content is in.
	ContentLanguage *string

	// Size of the body in bytes. This parameter is useful when the size of the body
	// cannot be determined automatically. For more information, see
	// https://www.rfc-editor.org/rfc/rfc9110.html#name-content-length (https://www.rfc-editor.org/rfc/rfc9110.html#name-content-length)
	// .
	ContentLength *int64

	// The base64-encoded 128-bit MD5 digest of the message (without the headers)
	// according to RFC 1864. This header can be used as a message integrity check to
	// verify that the data is the same data that was originally sent. Although it is
	// optional, we recommend using the Content-MD5 mechanism as an end-to-end
	// integrity check. For more information about REST request authentication, see
	// REST Authentication (https://docs.aws.amazon.com/AmazonS3/latest/dev/RESTAuthentication.html)
	// . The Content-MD5 header is required for any request to upload an object with a
	// retention period configured using Amazon S3 Object Lock. For more information
	// about Amazon S3 Object Lock, see Amazon S3 Object Lock Overview (https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html)
	// in the Amazon S3 User Guide. This functionality is not supported for directory
	// buckets.
	ContentMD5 *string

	// A standard MIME type describing the format of the contents. For more
	// information, see https://www.rfc-editor.org/rfc/rfc9110.html#name-content-type (https://www.rfc-editor.org/rfc/rfc9110.html#name-content-type)
	// .
	ContentType *string

	// The account ID of the expected bucket owner. If the account ID that you provide
	// does not match the actual owner of the bucket, the request fails with the HTTP
	// status code 403 Forbidden (access denied).
	ExpectedBucketOwner *string

	// The date and time at which the object is no longer cacheable. For more
	// information, see https://www.rfc-editor.org/rfc/rfc7234#section-5.3 (https://www.rfc-editor.org/rfc/rfc7234#section-5.3)
	// .
	Expires *time.Time

	// Gives the grantee READ, READ_ACP, and WRITE_ACP permissions on the object.
	//   - This functionality is not supported for directory buckets.
	//   - This functionality is not supported for Amazon S3 on Outposts.
	GrantFullControl *string

	// Allows grantee to read the object data and its metadata.
	//   - This functionality is not supported for directory buckets.
	//   - This functionality is not supported for Amazon S3 on Outposts.
	GrantRead *string

	// Allows grantee to read the object ACL.
	//   - This functionality is not supported for directory buckets.
	//   - This functionality is not supported for Amazon S3 on Outposts.
	GrantReadACP *string

	// Allows grantee to write the ACL for the applicable object.
	//   - This functionality is not supported for directory buckets.
	//   - This functionality is not supported for Amazon S3 on Outposts.
	GrantWriteACP *string

	// A map of metadata to store with the object in S3.
	Metadata map[string]string

	// Specifies whether a legal hold will be applied to this object. For more
	// information about S3 Object Lock, see Object Lock (https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock.html)
	// in the Amazon S3 User Guide. This functionality is not supported for directory
	// buckets.
	ObjectLockLegalHoldStatus types.ObjectLockLegalHoldStatus

	// The Object Lock mode that you want to apply to this object. This functionality
	// is not supported for directory buckets.
	ObjectLockMode types.ObjectLockMode

	// The date and time when you want this object's Object Lock to expire. Must be
	// formatted as a timestamp parameter. This functionality is not supported for
	// directory buckets.
	ObjectLockRetainUntilDate *time.Time

	// Confirms that the requester knows that they will be charged for the request.
	// Bucket owners need not specify this parameter in their requests. If either the
	// source or destination S3 bucket has Requester Pays enabled, the requester will
	// pay for corresponding charges to copy the object. For information about
	// downloading objects from Requester Pays buckets, see Downloading Objects in
	// Requester Pays Buckets (https://docs.aws.amazon.com/AmazonS3/latest/dev/ObjectsinRequesterPaysBuckets.html)
	// in the Amazon S3 User Guide. This functionality is not supported for directory
	// buckets.
	RequestPayer types.RequestPayer

	// Specifies the algorithm to use when encrypting the object (for example, AES256
	// ). This functionality is not supported for directory buckets.
	SSECustomerAlgorithm *string

	// Specifies the customer-provided encryption key for Amazon S3 to use in
	// encrypting data. This value is used to store the object and then it is
	// discarded; Amazon S3 does not store the encryption key. The key must be
	// appropriate for use with the algorithm specified in the
	// x-amz-server-side-encryption-customer-algorithm header. This functionality is
	// not supported for directory buckets.
	SSECustomerKey *string

	// Specifies the 128-bit MD5 digest of the encryption key according to RFC 1321.
	// Amazon S3 uses this header for a message integrity check to ensure that the
	// encryption key was transmitted without error. This functionality is not
	// supported for directory buckets.
	SSECustomerKeyMD5 *string

	// Specifies the Amazon Web Services KMS Encryption Context to use for object
	// encryption. The value of this header is a base64-encoded UTF-8 string holding
	// JSON with the encryption context key-value pairs. This value is stored as object
	// metadata and automatically gets passed on to Amazon Web Services KMS for future
	// GetObject or CopyObject operations on this object. This value must be
	// explicitly added during CopyObject operations. This functionality is not
	// supported for directory buckets.
	SSEKMSEncryptionContext *string

	// If x-amz-server-side-encryption has a valid value of aws:kms or aws:kms:dsse ,
	// this header specifies the ID (Key ID, Key ARN, or Key Alias) of the Key
	// Management Service (KMS) symmetric encryption customer managed key that was used
	// for the object. If you specify x-amz-server-side-encryption:aws:kms or
	// x-amz-server-side-encryption:aws:kms:dsse , but do not provide
	// x-amz-server-side-encryption-aws-kms-key-id , Amazon S3 uses the Amazon Web
	// Services managed key ( aws/s3 ) to protect the data. If the KMS key does not
	// exist in the same account that's issuing the command, you must use the full ARN
	// and not just the ID. This functionality is not supported for directory buckets.
	SSEKMSKeyId *string

	// The server-side encryption algorithm that was used when you store this object
	// in Amazon S3 (for example, AES256 , aws:kms , aws:kms:dsse ). General purpose
	// buckets - You have four mutually exclusive options to protect data using
	// server-side encryption in Amazon S3, depending on how you choose to manage the
	// encryption keys. Specifically, the encryption key options are Amazon S3 managed
	// keys (SSE-S3), Amazon Web Services KMS keys (SSE-KMS or DSSE-KMS), and
	// customer-provided keys (SSE-C). Amazon S3 encrypts data with server-side
	// encryption by using Amazon S3 managed keys (SSE-S3) by default. You can
	// optionally tell Amazon S3 to encrypt data at rest by using server-side
	// encryption with other key options. For more information, see Using Server-Side
	// Encryption (https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingServerSideEncryption.html)
	// in the Amazon S3 User Guide. Directory buckets - For directory buckets, only the
	// server-side encryption with Amazon S3 managed keys (SSE-S3) ( AES256 ) value is
	// supported.
	ServerSideEncryption types.ServerSideEncryption

	// By default, Amazon S3 uses the STANDARD Storage Class to store newly created
	// objects. The STANDARD storage class provides high durability and high
	// availability. Depending on performance needs, you can specify a different
	// Storage Class. For more information, see Storage Classes (https://docs.aws.amazon.com/AmazonS3/latest/dev/storage-class-intro.html)
	// in the Amazon S3 User Guide.
	//   - For directory buckets, only the S3 Express One Zone storage class is
	//   supported to store newly created objects.
	//   - Amazon S3 on Outposts only uses the OUTPOSTS Storage Class.
	StorageClass types.StorageClass

	// The tag-set for the object. The tag-set must be encoded as URL Query
	// parameters. (For example, "Key1=Value1") This functionality is not supported for
	// directory buckets.
	Tagging *string

	// If the bucket is configured as a website, redirects requests for this object to
	// another object in the same bucket or to an external URL. Amazon S3 stores the
	// value of this header in the object metadata. For information about object
	// metadata, see Object Key and Metadata (https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingMetadata.html)
	// in the Amazon S3 User Guide. In the following example, the request header sets
	// the redirect to an object (anotherPage.html) in the same bucket:
	// x-amz-website-redirect-location: /anotherPage.html In the following example, the
	// request header sets the object redirect to another website:
	// x-amz-website-redirect-location: http://www.example.com/ For more information
	// about website hosting in Amazon S3, see Hosting Websites on Amazon S3 (https://docs.aws.amazon.com/AmazonS3/latest/dev/WebsiteHosting.html)
	// and How to Configure Website Page Redirects (https://docs.aws.amazon.com/AmazonS3/latest/dev/how-to-page-redirect.html)
	// in the Amazon S3 User Guide. This functionality is not supported for directory
	// buckets.
	WebsiteRedirectLocation *string

	noSmithyDocumentSerde
}

func (in *PutObjectInput) bindEndpointParams(p *EndpointParameters) {
	p.Bucket = in.Bucket
	p.Key = in.Key

}

type PutObjectOutput struct {

	// Indicates whether the uploaded object uses an S3 Bucket Key for server-side
	// encryption with Key Management Service (KMS) keys (SSE-KMS). This functionality
	// is not supported for directory buckets.
	BucketKeyEnabled *bool

	// The base64-encoded, 32-bit CRC32 checksum of the object. This will only be
	// present if it was uploaded with the object. When you use an API operation on an
	// object that was uploaded using multipart uploads, this value may not be a direct
	// checksum value of the full object. Instead, it's a calculation based on the
	// checksum values of each individual part. For more information about how
	// checksums are calculated with multipart uploads, see Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html#large-object-checksums)
	// in the Amazon S3 User Guide.
	ChecksumCRC32 *string

	// The base64-encoded, 32-bit CRC32C checksum of the object. This will only be
	// present if it was uploaded with the object. When you use an API operation on an
	// object that was uploaded using multipart uploads, this value may not be a direct
	// checksum value of the full object. Instead, it's a calculation based on the
	// checksum values of each individual part. For more information about how
	// checksums are calculated with multipart uploads, see Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html#large-object-checksums)
	// in the Amazon S3 User Guide.
	ChecksumCRC32C *string

	// The base64-encoded, 160-bit SHA-1 digest of the object. This will only be
	// present if it was uploaded with the object. When you use the API operation on an
	// object that was uploaded using multipart uploads, this value may not be a direct
	// checksum value of the full object. Instead, it's a calculation based on the
	// checksum values of each individual part. For more information about how
	// checksums are calculated with multipart uploads, see Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html#large-object-checksums)
	// in the Amazon S3 User Guide.
	ChecksumSHA1 *string

	// The base64-encoded, 256-bit SHA-256 digest of the object. This will only be
	// present if it was uploaded with the object. When you use an API operation on an
	// object that was uploaded using multipart uploads, this value may not be a direct
	// checksum value of the full object. Instead, it's a calculation based on the
	// checksum values of each individual part. For more information about how
	// checksums are calculated with multipart uploads, see Checking object integrity (https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html#large-object-checksums)
	// in the Amazon S3 User Guide.
	ChecksumSHA256 *string

	// Entity tag for the uploaded object. General purpose buckets - To ensure that
	// data is not corrupted traversing the network, for objects where the ETag is the
	// MD5 digest of the object, you can calculate the MD5 while putting an object to
	// Amazon S3 and compare the returned ETag to the calculated MD5 value. Directory
	// buckets - The ETag for the object in a directory bucket isn't the MD5 digest of
	// the object.
	ETag *string

	// If the expiration is configured for the object (see
	// PutBucketLifecycleConfiguration (https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutBucketLifecycleConfiguration.html)
	// ) in the Amazon S3 User Guide, the response includes this header. It includes
	// the expiry-date and rule-id key-value pairs that provide information about
	// object expiration. The value of the rule-id is URL-encoded. This functionality
	// is not supported for directory buckets.
	Expiration *string

	// If present, indicates that the requester was successfully charged for the
	// request. This functionality is not supported for directory buckets.
	RequestCharged types.RequestCharged

	// If server-side encryption with a customer-provided encryption key was
	// requested, the response will include this header to confirm the encryption
	// algorithm that's used. This functionality is not supported for directory
	// buckets.
	SSECustomerAlgorithm *string

	// If server-side encryption with a customer-provided encryption key was
	// requested, the response will include this header to provide the round-trip
	// message integrity verification of the customer-provided encryption key. This
	// functionality is not supported for directory buckets.
	SSECustomerKeyMD5 *string

	// If present, indicates the Amazon Web Services KMS Encryption Context to use for
	// object encryption. The value of this header is a base64-encoded UTF-8 string
	// holding JSON with the encryption context key-value pairs. This value is stored
	// as object metadata and automatically gets passed on to Amazon Web Services KMS
	// for future GetObject or CopyObject operations on this object. This
	// functionality is not supported for directory buckets.
	SSEKMSEncryptionContext *string

	// If x-amz-server-side-encryption has a valid value of aws:kms or aws:kms:dsse ,
	// this header indicates the ID of the Key Management Service (KMS) symmetric
	// encryption customer managed key that was used for the object. This functionality
	// is not supported for directory buckets.
	SSEKMSKeyId *string

	// The server-side encryption algorithm used when you store this object in Amazon
	// S3 (for example, AES256 , aws:kms , aws:kms:dsse ). For directory buckets, only
	// server-side encryption with Amazon S3 managed keys (SSE-S3) ( AES256 ) is
	// supported.
	ServerSideEncryption types.ServerSideEncryption

	// Version ID of the object. If you enable versioning for a bucket, Amazon S3
	// automatically generates a unique version ID for the object being stored. Amazon
	// S3 returns this ID in the response. When you enable versioning for a bucket, if
	// Amazon S3 receives multiple write requests for the same object simultaneously,
	// it stores all of the objects. For more information about versioning, see Adding
	// Objects to Versioning-Enabled Buckets (https://docs.aws.amazon.com/AmazonS3/latest/dev/AddingObjectstoVersioningEnabledBuckets.html)
	// in the Amazon S3 User Guide. For information about returning the versioning
	// state of a bucket, see GetBucketVersioning (https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetBucketVersioning.html)
	// . This functionality is not supported for directory buckets.
	VersionId *string

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationPutObjectMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsRestxml_serializeOpPutObject{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsRestxml_deserializeOpPutObject{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "PutObject"); err != nil {
		return fmt.Errorf("add protocol finalizers: %v", err)
	}

	if err = addlegacyEndpointContextSetter(stack, options); err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = addClientRequestID(stack); err != nil {
		return err
	}
	if err = addComputeContentLength(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = addComputePayloadSHA256(stack); err != nil {
		return err
	}
	if err = addRetry(stack, options); err != nil {
		return err
	}
	if err = addRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = addRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addClientUserAgent(stack, options); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addSetLegacyContextSigningOptionsMiddleware(stack); err != nil {
		return err
	}
	if err = addPutBucketContextMiddleware(stack); err != nil {
		return err
	}
	if err = addOpPutObjectValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opPutObject(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addMetadataRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = add100Continue(stack, options); err != nil {
		return err
	}
	if err = addRecursionDetection(stack); err != nil {
		return err
	}
	if err = addPutObjectInputChecksumMiddlewares(stack, options); err != nil {
		return err
	}
	if err = addPutObjectUpdateEndpoint(stack, options); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = v4.AddContentSHA256HeaderMiddleware(stack); err != nil {
		return err
	}
	if err = v4.UseDynamicPayloadSigningMiddleware(stack); err != nil {
		return err
	}
	if err = disableAcceptEncodingGzip(stack); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	if err = addDisableHTTPSMiddleware(stack, options); err != nil {
		return err
	}
	if err = addSerializeImmutableHostnameBucketMiddleware(stack, options); err != nil {
		return err
	}
	return nil
}

func (v *PutObjectInput) bucket() (string, bool) {
	if v.Bucket == nil {
		return "", false
	}
	return *v.Bucket, true
}

func newServiceMetadataMiddleware_opPutObject(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "PutObject",
	}
}

// getPutObjectRequestAlgorithmMember gets the request checksum algorithm value
// provided as input.
func getPutObjectRequestAlgorithmMember(input interface{}) (string, bool) {
	in := input.(*PutObjectInput)
	if len(in.ChecksumAlgorithm) == 0 {
		return "", false
	}
	return string(in.ChecksumAlgorithm), true
}

func addPutObjectInputChecksumMiddlewares(stack *middleware.Stack, options Options) error {
	return internalChecksum.AddInputMiddleware(stack, internalChecksum.InputMiddlewareOptions{
		GetAlgorithm:                     getPutObjectRequestAlgorithmMember,
		RequireChecksum:                  false,
		EnableTrailingChecksum:           true,
		EnableComputeSHA256PayloadHash:   true,
		EnableDecodedContentLengthHeader: true,
	})
}

// getPutObjectBucketMember returns a pointer to string denoting a provided bucket
// member valueand a boolean indicating if the input has a modeled bucket name,
func getPutObjectBucketMember(input interface{}) (*string, bool) {
	in := input.(*PutObjectInput)
	if in.Bucket == nil {
		return nil, false
	}
	return in.Bucket, true
}
func addPutObjectUpdateEndpoint(stack *middleware.Stack, options Options) error {
	return s3cust.UpdateEndpoint(stack, s3cust.UpdateEndpointOptions{
		Accessor: s3cust.UpdateEndpointParameterAccessor{
			GetBucketFromInput: getPutObjectBucketMember,
		},
		UsePathStyle:                   options.UsePathStyle,
		UseAccelerate:                  options.UseAccelerate,
		SupportsAccelerate:             true,
		TargetS3ObjectLambda:           false,
		EndpointResolver:               options.EndpointResolver,
		EndpointResolverOptions:        options.EndpointOptions,
		UseARNRegion:                   options.UseARNRegion,
		DisableMultiRegionAccessPoints: options.DisableMultiRegionAccessPoints,
	})
}

// PresignPutObject is used to generate a presigned HTTP Request which contains
// presigned URL, signed headers and HTTP method used.
func (c *PresignClient) PresignPutObject(ctx context.Context, params *PutObjectInput, optFns ...func(*PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	if params == nil {
		params = &PutObjectInput{}
	}
	options := c.options.copy()
	for _, fn := range optFns {
		fn(&options)
	}
	clientOptFns := append(options.ClientOptions, withNopHTTPClientAPIOption)

	result, _, err := c.client.invokeOperation(ctx, "PutObject", params, clientOptFns,
		c.client.addOperationPutObjectMiddlewares,
		presignConverter(options).convertToPresignMiddleware,
		func(stack *middleware.Stack, options Options) error {
			return awshttp.RemoveContentTypeHeader(stack)
		},
		addPutObjectPayloadAsUnsigned,
	)
	if err != nil {
		return nil, err
	}

	out := result.(*v4.PresignedHTTPRequest)
	return out, nil
}

func addPutObjectPayloadAsUnsigned(stack *middleware.Stack, options Options) error {
	v4.RemoveContentSHA256HeaderMiddleware(stack)
	v4.RemoveComputePayloadSHA256Middleware(stack)
	return v4.AddUnsignedPayloadMiddleware(stack)
}
