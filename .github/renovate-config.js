module.exports = {
  platform: 'github',
  branchPrefix: 'renovate/',
  username: 'renovate-release',
  gitAuthor: 'Renovate Bot <bot@renovateapp.com>',
  onboardingConfig: {
    extends: ['config:base'],
  },
  repositories: ['openstack-k8s-operators/openstack-operator']
};
