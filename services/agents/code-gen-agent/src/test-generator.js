// Test script to demonstrate code generation capabilities
const { CodeGenerator } = require('./codeGenerator');

const generator = new CodeGenerator();

console.log('Testing Code Generator\n');

// Test 1: Generate REST API
console.log('1. Generating REST API:');
const restAPI = generator.generateRestAPI({
  endpoint: '/api/products',
  methods: ['GET', 'POST'],
  modelName: 'Product',
  fields: [
    { name: 'name', type: 'string', required: true },
    { name: 'price', type: 'number', required: true }
  ]
});
console.log(restAPI.files[0].content);
console.log('\n---\n');

// Test 2: Generate React Component
console.log('2. Generating React Component:');
const reactComponent = generator.generateReactComponent({
  name: 'ProductCard',
  type: 'functional',
  props: ['product', 'onAddToCart'],
  useState: true
});
console.log(reactComponent.files[0].content);
console.log('\n---\n');

// Test 3: Generate Express Server
console.log('3. Generating Express Server:');
const expressServer = generator.generateExpressServer({
  port: 5000,
  cors: true,
  helmet: true,
  routes: [
    { path: '/api/products', file: 'products' },
    { path: '/api/users', file: 'users' }
  ]
});
console.log(expressServer.files[0].content);
console.log('\n---\n');

// Test 4: Generate Mongoose Schema
console.log('4. Generating Mongoose Schema:');
const mongooseSchema = generator.generateDatabaseSchema({
  orm: 'mongoose',
  modelName: 'User',
  fields: [
    { name: 'username', type: 'string', required: true, unique: true },
    { name: 'email', type: 'string', required: true, unique: true },
    { name: 'isActive', type: 'boolean', default: true }
  ]
});
console.log(mongooseSchema.files[0].content);
console.log('\n---\n');

// Test 5: Custom prompt generation
console.log('5. Generating from custom prompt:');
const customCode = generator.generateFromPrompt({
  prompt: 'Create a function to validate user input',
  language: 'javascript'
});
console.log(customCode.files[0].content);

console.log('\nAll tests completed!');